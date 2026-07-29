import { Navigate, useLocation, useSearchParams } from "react-router-dom";
import { Button } from "@oneapply/ui";
import { useAuth } from "../lib/auth";

export function Login() {
  const { signIn, isAuthenticated, isLoading } = useAuth();
  const location = useLocation();
  const [params] = useSearchParams();
  const error = params.get("error");

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center text-gray-500 text-sm">
        Loading…
      </div>
    );
  }
  if (isAuthenticated) {
    const from = (location.state as { from?: { pathname: string } } | null)?.from?.pathname ?? "/outreach";
    return <Navigate to={from} replace />;
  }

  return (
    <div className="flex h-full items-center justify-center">
      <div className="w-full max-w-sm rounded-lg border border-gray-200 bg-white p-8 shadow-sm">
        <h1 className="text-xl font-semibold mb-2">OneApply</h1>
        <p className="text-sm text-gray-600 mb-6">
          One click to reach any recruiter.
        </p>

        {error && (
          <div className="mb-4 rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            Google returned: {error}
          </div>
        )}

        <Button
          variant="primary"
          className="w-full"
          onClick={signIn}
        >
          Sign in with Google
        </Button>
        <p className="mt-4 text-xs text-gray-500">
          We&apos;ll ask for permission to send email on your behalf and read
          replies so we can track recruiter responses.
        </p>
      </div>
    </div>
  );
}
