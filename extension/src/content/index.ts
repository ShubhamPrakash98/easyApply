// Content script: injects a "Reach Out (OneApply)" button on LinkedIn
// recruiter profile pages. On click it scrapes visible profile fields,
// hands them to the background service worker, and asks it to open
// the extension popup for the draft/approve flow.
//
// Extraction cascade (in order of resilience). Modern logged-in LinkedIn strips
// most structured metadata (no JSON-LD, no og: tags, no <h1>) so we lean on
// text-based signals that they must keep intact for the app to work at all:
//   1) Schema.org Person JSON-LD — kept for completeness; only present on
//      logged-out / crawler-facing views but harmless to try.
//   2) og:title / og:description — same as above.
//   3) document.title — LinkedIn updates this on route change even in the SPA
//      view. Format: "(N) Name | LinkedIn" — strip prefix + suffix.
//   4) body.innerText parsing — locate the name in visible text, take the
//      following non-nav line as the headline. Robust to any CSS changes.
//   5) DOM h1 selector chain — legacy fallback.

const BTN_ID = "oneapply-reach-out-btn";
const TOAST_ID = "oneapply-toast";
const LOG_PREFIX = "[OneApply]";

interface CapturedProfile {
  recruiter_name: string;
  recruiter_headline: string;
  company: string;
  linkedin_url: string;
}

/** Try each selector in order; return the first non-empty result.
 *  Uses `innerText || textContent` (not ??) so an empty innerText from a
 *  hidden or hydrating element still falls through to textContent. */
function trySelectors(
  selectors: string[],
  attrName?: string,
): { value: string; matched: string | null } {
  for (const sel of selectors) {
    const el = document.querySelector<HTMLElement>(sel);
    if (!el) continue;
    const raw = attrName
      ? el.getAttribute(attrName)
      : el.innerText || el.textContent;
    const v = (raw ?? "").trim();
    if (v) return { value: v, matched: sel };
  }
  return { value: "", matched: null };
}

// -----------------------------------------------------------------------------
// Schema.org JSON-LD extraction — LinkedIn's own structured data.
// -----------------------------------------------------------------------------

interface PersonJSONLD {
  name?: string;
  jobTitle?: string;
  worksFor?: JSONLDNode | JSONLDNode[];
}

interface JSONLDNode {
  "@type"?: string | string[];
  name?: string;
  jobTitle?: string;
  worksFor?: JSONLDNode | JSONLDNode[];
  mainEntity?: JSONLDNode;
  mainEntityOfPage?: JSONLDNode;
  "@graph"?: JSONLDNode[];
  [k: string]: unknown;
}

/** Deep-walk any Schema.org JSON-LD payload and return the first Person node. */
function findPerson(node: JSONLDNode | JSONLDNode[] | undefined): PersonJSONLD | null {
  if (!node) return null;
  if (Array.isArray(node)) {
    for (const n of node) {
      const p = findPerson(n);
      if (p) return p;
    }
    return null;
  }
  const t = node["@type"];
  const isPerson =
    (typeof t === "string" && t === "Person") ||
    (Array.isArray(t) && t.includes("Person"));
  if (isPerson && typeof node.name === "string" && node.name) return node as PersonJSONLD;

  // Recurse into common wrapper keys.
  for (const key of ["mainEntity", "mainEntityOfPage", "@graph"] as const) {
    const p = findPerson(node[key] as JSONLDNode | JSONLDNode[] | undefined);
    if (p) return p;
  }
  return null;
}

interface ProfileFromJSONLD {
  name: string;
  jobTitle: string;
  company: string;
}

function readJSONLD(): ProfileFromJSONLD | null {
  const scripts = document.querySelectorAll<HTMLScriptElement>(
    'script[type="application/ld+json"]',
  );
  for (const s of scripts) {
    try {
      const payload = JSON.parse(s.textContent || "null") as JSONLDNode | JSONLDNode[] | null;
      if (!payload) continue;
      const person = findPerson(payload);
      if (!person) continue;

      // worksFor may be an object or array; take the first with a name.
      let company = "";
      const wf = person.worksFor;
      if (Array.isArray(wf)) {
        for (const w of wf) {
          if (typeof w?.name === "string" && w.name) {
            company = w.name;
            break;
          }
        }
      } else if (wf && typeof wf.name === "string") {
        company = wf.name;
      }

      return {
        name: person.name ?? "",
        jobTitle: person.jobTitle ?? "",
        company,
      };
    } catch {
      // Not valid JSON — skip.
    }
  }
  return null;
}

/** og:title is always in the initial HTML head, so it survives SPA hydration.
 *  LinkedIn formats are "Name | Headline | LinkedIn", "Name - LinkedIn",
 *  or just "Name | Company | LinkedIn" — take the first piece. */
function nameFromOgTitle(): string {
  const og =
    document.querySelector<HTMLMetaElement>('meta[property="og:title"]')?.content ?? "";
  if (!og) return "";
  const first = og.split(/[|·]/)[0].trim();
  return first.replace(/\s*[-–]\s*LinkedIn.*$/i, "").trim();
}

function headlineFromOgDescription(): string {
  const og =
    document.querySelector<HTMLMetaElement>('meta[property="og:description"]')?.content ?? "";
  return og.trim();
}

// -----------------------------------------------------------------------------
// Text-based extraction — works even when LinkedIn strips all metadata.
// -----------------------------------------------------------------------------

/** Extract name from document.title.
 *  Common formats: "(3) Tim Zheng | LinkedIn", "Tim Zheng | LinkedIn",
 *  "Tim Zheng - LinkedIn", "(1) Tim Zheng - Founder & CEO at ... | LinkedIn". */
function nameFromDocumentTitle(): string {
  const title = document.title ?? "";
  if (!title) return "";
  // Strip "(N)" or "(N+)" notification-count prefix.
  let s = title.replace(/^\s*\(\d+\+?\)\s*/, "");
  // Strip " | LinkedIn" / " – LinkedIn" / " - LinkedIn" suffix.
  s = s.replace(/\s*[|–\-]\s*LinkedIn\s*$/i, "");
  // If what's left contains "|" or " - " with more info, keep the first chunk.
  s = s.split(/\s*[|–]\s*/)[0];
  return s.trim();
}

/** Lines LinkedIn's chrome adds around the profile content.
 *  Anything matching these is skipped when we're looking for the headline. */
const LI_CHROME_PATTERNS: RegExp[] = [
  /^(more|message|follow|connect|send|save|share|about|reach out|reach out \(oneapply\))$/i,
  /^\d+$/, // pure numbers (notification counts)
  /^(home|my network|jobs|messaging|notifications|me|for business)$/i,
  /^skip to /i,
  /^try premium/i,
  /^\d+\s*notification/i,
  /^see all/i,
  /^view /i,
];

function isChromeLine(line: string): boolean {
  return LI_CHROME_PATTERNS.some((r) => r.test(line));
}

/** Once we know the name, find the profile headline in body innerText.
 *  Strategy: find the line matching the name, then walk forward skipping
 *  LinkedIn chrome, and return the first substantive line. */
function headlineFromBodyText(name: string): string {
  if (!name) return "";
  const body = document.body?.innerText ?? "";
  if (!body) return "";
  const lines = body.split(/\n+/).map((l) => l.trim()).filter(Boolean);
  const idx = lines.findIndex((l) => l === name || l.startsWith(name));
  if (idx < 0) return "";
  for (let i = idx + 1; i < Math.min(idx + 12, lines.length); i++) {
    const line = lines[i];
    if (isChromeLine(line)) continue;
    if (line.length < 4) continue;
    // Reasonable headline length
    return line.slice(0, 240);
  }
  return "";
}

/** Best-effort company extraction from the headline. LinkedIn headlines look
 *  like "Co-Founder & Chairman at Apollo.io | Building something new" or
 *  "Senior Recruiter at Google". */
function companyFromHeadline(headline: string): string {
  if (!headline) return "";
  // Grab text after " at " up to a separator ("|", "·", or end).
  const m = headline.match(/\bat\s+([^|·]+?)(\s*[|·].*)?$/i);
  if (!m) return "";
  return m[1].trim();
}

function readProfile(): CapturedProfile {
  let name = "";
  let headline = "";
  let company = "";
  const matched = { name: null as string | null, headline: null as string | null, company: null as string | null };

  // Layer 1 — Schema.org JSON-LD (primary source; stable, structured).
  const ld = readJSONLD();
  if (ld) {
    if (ld.name) {
      name = ld.name;
      matched.name = "json-ld";
    }
    if (ld.jobTitle) {
      headline = ld.jobTitle;
      matched.headline = "json-ld";
    }
    if (ld.company) {
      company = ld.company;
      matched.company = "json-ld";
    }
  }

  // Layer 2 — og:title / og:description meta tags.
  if (!name) {
    const n = nameFromOgTitle();
    if (n) { name = n; matched.name = "og:title"; }
  }
  if (!headline) {
    const h = headlineFromOgDescription();
    if (h) { headline = h; matched.headline = "og:description"; }
  }

  // Layer 3 — document.title. Reliable in the logged-in SPA view.
  if (!name) {
    const n = nameFromDocumentTitle();
    if (n) { name = n; matched.name = "document.title"; }
  }

  // Layer 4 — body.innerText, anchored on the name.
  if (name && !headline) {
    const h = headlineFromBodyText(name);
    if (h) { headline = h; matched.headline = "body-text"; }
  }

  // Layer 5 — DOM selectors (last resort, brittle; keep for old layouts).
  if (!name) {
    const nameHit = trySelectors([
      "h1.text-heading-xlarge",
      "h1.top-card-layout__title",
      "main section h1",
      "section.pv-top-card h1",
      ".pv-text-details__left-panel h1",
      ".ph5 h1",
      "h1",
    ]);
    name = nameHit.value;
    matched.name = nameHit.matched;
  }
  if (!headline) {
    const headlineHit = trySelectors([
      ".text-body-medium.break-words",
      ".pv-text-details__left-panel .text-body-medium",
      "section.pv-top-card div.text-body-medium",
      "div.text-body-medium",
    ]);
    headline = headlineHit.value;
    matched.headline = headlineHit.matched;
  }

  // Company — try aria-label, chips, then parse from headline.
  if (!company) {
    const companyHit = trySelectors(
      [
        '[data-field="experience_company_logo"]',
        'button[aria-label*="company" i]',
      ],
      "aria-label",
    );
    company = companyHit.value.replace(/\s*current company:?\s*/i, "").trim();
    matched.company = companyHit.matched;
  }
  if (!company) {
    const chip = trySelectors([
      ".pv-text-details__right-panel .inline-show-more-text",
      "ul.pv-text-details__right-panel-items li",
    ]);
    if (chip.value) { company = chip.value; matched.company = chip.matched; }
  }
  if (!company && headline) {
    const c = companyFromHeadline(headline);
    if (c) { company = c; matched.company = "headline-parse"; }
  }

  const linkedin_url = window.location.href.split("?")[0];

  console.log(`${LOG_PREFIX} scraped`, {
    name,
    headline,
    company,
    linkedin_url,
    matched,
  });

  return {
    recruiter_name: name,
    recruiter_headline: headline,
    company,
    linkedin_url,
  };
}

function showToast(message: string) {
  document.getElementById(TOAST_ID)?.remove();
  const el = document.createElement("div");
  el.id = TOAST_ID;
  el.textContent = message;
  el.style.cssText = [
    "position: fixed",
    "bottom: 24px",
    "right: 24px",
    "z-index: 2147483647",
    "background: #111827",
    "color: white",
    "padding: 12px 18px",
    "border-radius: 8px",
    "font-family: system-ui, sans-serif",
    "font-size: 13px",
    "box-shadow: 0 10px 20px rgba(0,0,0,0.2)",
  ].join(";");
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 4500);
}

function injectButton() {
  if (document.getElementById(BTN_ID)) return;

  const anchor =
    document.querySelector<HTMLDivElement>(".pv-top-card-v2-ctas") ??
    document.querySelector<HTMLElement>("main");
  if (!anchor) return;

  const btn = document.createElement("button");
  btn.id = BTN_ID;
  btn.textContent = "Reach Out (OneApply)";
  btn.style.cssText = [
    "margin: 8px 0",
    "padding: 8px 14px",
    "background: #4f46e5",
    "color: white",
    "border: none",
    "border-radius: 6px",
    "font-weight: 500",
    "cursor: pointer",
    "font-size: 14px",
  ].join(";");
  btn.addEventListener("click", async () => {
    const profile = readProfile();
    if (!profile.recruiter_name) {
      showToast("OneApply: couldn't read recruiter name. Check DevTools console for what was found.");
      return;
    }
    try {
      const resp = await chrome.runtime.sendMessage({
        type: "REACH_OUT_CLICKED",
        profile,
      });
      if (resp?.opened) {
        // popup opened — nothing else to do here
      } else {
        showToast("OneApply: click the extension icon to draft →");
      }
    } catch (err) {
      console.error(`${LOG_PREFIX} send message failed`, err);
      showToast("OneApply: extension not responding. Reload the page.");
    }
  });

  anchor.prepend(btn);
}

const observer = new MutationObserver(() => injectButton());
observer.observe(document.body, { childList: true, subtree: true });
injectButton();
