import { Button } from "@oneapply/ui";

export function Login() {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="w-full max-w-sm rounded-lg border border-gray-200 bg-white p-8 shadow-sm">
        <h1 className="text-xl font-semibold mb-2">OneApply</h1>
        <p className="text-sm text-gray-600 mb-6">
          One click to reach any recruiter.
        </p>
        <Button
          variant="primary"
          className="w-full"
          disabled
          title="Enabled in Phase 1"
        >
          Sign in with Google
        </Button>
        <p className="mt-4 text-xs text-gray-500">
          Google OAuth wires up in Phase 1.
        </p>
      </div>
    </div>
  );
}
