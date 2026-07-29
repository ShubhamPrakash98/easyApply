// MV3 service worker.
// Phase 6: subscribe to backend SSE for reply notifications.
// Phase 0: just log messages from the content script so we know wiring works.

chrome.runtime.onInstalled.addListener(() => {
  console.log("[OneApply] extension installed");
});

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  console.log("[OneApply] bg received", msg, "from", sender.tab?.url);
  sendResponse({ ok: true });
  return true;
});
