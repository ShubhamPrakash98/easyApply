import { Button } from "@oneapply/ui";

export function Popup() {
  return (
    <div className="w-80 p-4 space-y-3 font-sans">
      <div>
        <div className="text-lg font-semibold">OneApply</div>
        <div className="text-xs text-gray-500">One click to reach any recruiter.</div>
      </div>

      <div className="rounded border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700">
        Open a LinkedIn recruiter profile and click <span className="font-medium">Reach Out</span>.
      </div>

      <Button variant="secondary" size="sm" className="w-full" disabled>
        Sign in (Phase 1)
      </Button>
    </div>
  );
}
