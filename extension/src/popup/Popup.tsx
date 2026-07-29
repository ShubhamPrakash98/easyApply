import { Button } from "@oneapply/ui";
import { useAuth } from "./useAuth";

export function Popup() {
  const { status, user, error, signIn, signOut } = useAuth();

  return (
    <div className="w-80 p-4 space-y-3 font-sans">
      <div>
        <div className="text-lg font-semibold">OneApply</div>
        <div className="text-xs text-gray-500">
          One click to reach any recruiter.
        </div>
      </div>

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
          <div className="rounded border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700">
            Signed in as{" "}
            <span className="font-medium text-gray-900">{user.email}</span>.
            {user.gmail_connected
              ? " Gmail connected."
              : " Gmail scope pending."}
          </div>
          <div className="rounded border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700">
            Open a LinkedIn recruiter profile and click{" "}
            <span className="font-medium">Reach Out</span>.
          </div>
          <Button
            variant="secondary"
            size="sm"
            className="w-full"
            onClick={() => void signOut()}
          >
            Sign out
          </Button>
        </>
      )}
    </div>
  );
}
