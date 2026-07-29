// Content script: injects a "Reach Out (OneApply)" button on LinkedIn
// recruiter profile pages. On click it scrapes visible profile fields,
// hands them to the background service worker, and asks it to open
// the extension popup for the draft/approve flow.

const BTN_ID = "oneapply-reach-out-btn";
const TOAST_ID = "oneapply-toast";

interface CapturedProfile {
  recruiter_name: string;
  recruiter_headline: string;
  company: string;
  linkedin_url: string;
}

function readProfile(): CapturedProfile {
  const name =
    document.querySelector<HTMLHeadingElement>("h1.text-heading-xlarge")?.innerText.trim() ??
    document.querySelector<HTMLHeadingElement>("h1")?.innerText.trim() ??
    "";

  const headline =
    document.querySelector<HTMLDivElement>(".text-body-medium.break-words")?.innerText.trim() ?? "";

  // Best-effort company extraction from the top-card experience section.
  let company =
    document
      .querySelector<HTMLDivElement>('[data-field="experience_company_logo"]')
      ?.getAttribute("aria-label") ?? "";

  if (!company) {
    // Try the current-company chip near the top card.
    const chip = document.querySelector<HTMLDivElement>(".pv-text-details__right-panel .inline-show-more-text");
    if (chip) company = chip.innerText.trim();
  }

  // If headline still has "at X", pull X out as a company fallback.
  if (!company && headline.toLowerCase().includes(" at ")) {
    company = headline.split(/ at /i).pop()?.trim() ?? "";
  }

  return {
    recruiter_name: name,
    recruiter_headline: headline,
    company,
    linkedin_url: window.location.href.split("?")[0],
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
      showToast("OneApply: couldn't read recruiter name from this page.");
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
      console.error("[OneApply] send message failed", err);
      showToast("OneApply: extension not responding. Reload the page.");
    }
  });

  anchor.prepend(btn);
}

const observer = new MutationObserver(() => injectButton());
observer.observe(document.body, { childList: true, subtree: true });
injectButton();
