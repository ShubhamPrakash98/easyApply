// MV3 service worker.
// Responsibilities so far:
//  - accept REACH_OUT_CLICKED from content script
//  - persist the captured profile in chrome.storage.session
//  - try to open the popup so the user can immediately draft
//
// Phase 6 will add SSE subscription for reply notifications.

const PENDING_KEY = "pending_capture";

chrome.runtime.onInstalled.addListener(() => {
  console.log("[OneApply] extension installed");
});

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg?.type !== "REACH_OUT_CLICKED") {
    return false;
  }

  const profile = msg.profile;
  console.log("[OneApply] captured", profile, "from", sender.tab?.url);

  (async () => {
    await chrome.storage.session.set({ [PENDING_KEY]: { profile, capturedAt: Date.now() } });
    let opened = false;
    try {
      // chrome.action.openPopup() requires user gesture. The gesture from
      // the LinkedIn button click propagates here, but browsers can still
      // deny — swallow errors and let the content script show a toast.
      await chrome.action.openPopup();
      opened = true;
    } catch (err) {
      console.warn("[OneApply] openPopup failed (expected in some browser states)", err);
    }
    sendResponse({ ok: true, opened });
  })();

  // Signal async response.
  return true;
});
