// Content script: injects a "Reach Out" button on LinkedIn profile pages.
// Phase 0 goal: prove injection works. Phase 2 wires up the actual capture flow.

const BTN_ID = "oneapply-reach-out-btn";

function readProfile() {
  const name =
    document.querySelector<HTMLHeadingElement>("h1.text-heading-xlarge")?.innerText.trim() ??
    document.querySelector<HTMLHeadingElement>("h1")?.innerText.trim() ??
    "";
  const headline =
    document.querySelector<HTMLDivElement>(".text-body-medium.break-words")?.innerText.trim() ?? "";
  const company =
    document
      .querySelector<HTMLDivElement>('[data-field="experience_company_logo"]')
      ?.getAttribute("aria-label") ?? "";

  return {
    name,
    headline,
    company,
    linkedin_url: window.location.href.split("?")[0],
  };
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
  btn.addEventListener("click", () => {
    const profile = readProfile();
    console.log("[OneApply] captured", profile);
    chrome.runtime.sendMessage({ type: "REACH_OUT_CLICKED", profile });
    alert(`OneApply captured:\n\n${JSON.stringify(profile, null, 2)}\n\n(Draft flow in Phase 2.)`);
  });

  anchor.prepend(btn);
}

const observer = new MutationObserver(() => injectButton());
observer.observe(document.body, { childList: true, subtree: true });
injectButton();
