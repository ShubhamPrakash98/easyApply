import { useEffect, useState } from "react";
import { Button } from "@oneapply/ui";
import type { CapturedProfile } from "@oneapply/api-client";
import { useAuth } from "./useAuth";
import { DraftView } from "./DraftView";

const PENDING_KEY = "pending_capture";

interface PendingCapture {
  profile: CapturedProfile;
  capturedAt: number;
}

export function Popup() {
  const { status, user, error, signIn, signOut } = useAuth();
  const [pending, setPending] = useState<PendingCapture | null>(null);

  useEffect(() => {
    let mounted = true;
    void chrome.storage.session.get(PENDING_KEY).then((data) => {
      if (mounted) setPending((data[PENDING_KEY] as PendingCapture | undefined) ?? null);
    });
    const listener = (
      changes: { [key: string]: chrome.storage.StorageChange },
      area: string,
    ) => {
      if (area !== "session" || !(PENDING_KEY in changes)) return;
      setPending((changes[PENDING_KEY].newValue as PendingCapture | null) ?? null);
    };
    chrome.storage.onChanged.addListener(listener);
    return () => {
      mounted = false;
      chrome.storage.onChanged.removeListener(listener);
    };
  }, []);

  async function clearPending() {
    await chrome.storage.session.remove(PENDING_KEY);
    setPending(null);
  }

  return (
    <div className="w-96 p-4 space-y-3 font-sans">
      <header>
        <div className="text-lg font-semibold">OneApply</div>
        <div className="text-xs text-gray-500">
          One click to reach any recruiter.
        </div>
      </header>

      {status === "loading" && (
        <div className="text-sm text-gray-500">Checking session…</div>
      )}

      {status === "error" && (
        <div className="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">
          {error ?? "Something went wrong"}
        </div>
      )}

      {status === "unauthenticated" && (
        <>
          <div className="rounded border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700">
            Sign in to draft and send emails through your Gmail account.
          </div>
          <Button variant="primary" className="w-full" onClick={signIn}>
            Sign in with Google
          </Button>
          <p className="text-xs text-gray-500">
            Opens the dashboard in a new tab. Return here after signing in.
          </p>
        </>
      )}

      {status === "authenticated" && user && (
        <>
          <div className="flex items-center justify-between rounded border border-gray-200 bg-gray-50 p-2 text-xs text-gray-700">
            <span className="truncate" title={user.email}>{user.email}</span>
            <button
              onClick={() => void signOut()}
              className="ml-2 text-gray-500 hover:text-gray-900"
            >
              Sign out
            </button>
          </div>

          {pending ? (
            <DraftView
              profile={pending.profile}
              onDone={() => void clearPending()}
              onDismiss={() => window.close()}
            />
          ) : (
            <div className="rounded border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700">
              Open a LinkedIn recruiter profile and click{" "}
              <span className="font-medium">Reach Out (OneApply)</span>.
            </div>
          )}
        </>
      )}
    </div>
  );
}
