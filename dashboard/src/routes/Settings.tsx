import { useQuery } from "@tanstack/react-query";
import type { HealthResponse } from "@oneapply/api-client";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";

const featureLabels: Record<string, { title: string; blurb: string }> = {
  ai_draft_email: {
    title: "AI-drafted outreach emails",
    blurb: "Claude writes a personalized draft from the JD + your resume.",
  },
  ai_resume_match: {
    title: "AI resume matching",
    blurb: "Auto-picks the best resume variant for each JD.",
  },
  ai_followup: {
    title: "AI follow-ups",
    blurb: "Automatically drafts and sends 3 follow-ups if no reply.",
  },
};

export function Settings() {
  const { user } = useAuth();
  const { data, isLoading, error } = useQuery({
    queryKey: ["health"],
    queryFn: () => api.get<HealthResponse>("/health"),
  });

  return (
    <section className="max-w-2xl space-y-8">
      <h1 className="text-2xl font-semibold">Settings</h1>

      {user && (
        <div className="rounded border border-gray-200 bg-white p-4">
          <div className="text-sm font-medium mb-3">Account</div>
          <div className="space-y-1 text-sm">
            <div>
              <span className="text-gray-500">Email:</span>{" "}
              <span className="font-medium">{user.email}</span>
            </div>
            <div>
              <span className="text-gray-500">Gmail connected:</span>{" "}
              {user.gmail_connected ? (
                <span className="text-green-700">yes</span>
              ) : (
                <span className="text-red-700">no — sign out and back in</span>
              )}
            </div>
            <div>
              <span className="text-gray-500">Plan:</span>{" "}
              <span
                className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${
                  user.subscription_tier === "premium"
                    ? "bg-indigo-100 text-indigo-800"
                    : "bg-gray-100 text-gray-700"
                }`}
              >
                {user.subscription_tier}
              </span>
            </div>
          </div>
        </div>
      )}

      {user && (
        <div className="rounded border border-gray-200 bg-white p-4">
          <div className="text-sm font-medium mb-3">Features</div>
          <ul className="space-y-3">
            {Object.entries(user.features).map(([key, enabled]) => {
              const meta = featureLabels[key] ?? { title: key, blurb: "" };
              return (
                <li key={key} className="flex items-start gap-3">
                  <span
                    className={`mt-0.5 inline-flex h-5 shrink-0 items-center rounded px-2 text-xs font-medium ${
                      enabled
                        ? "bg-green-100 text-green-800"
                        : "bg-gray-100 text-gray-600"
                    }`}
                  >
                    {enabled ? "on" : "off"}
                  </span>
                  <div>
                    <div className="text-sm font-medium text-gray-900">{meta.title}</div>
                    <div className="text-xs text-gray-500">{meta.blurb}</div>
                  </div>
                </li>
              );
            })}
          </ul>
          {user.subscription_tier === "free" && (
            <div className="mt-4 rounded border border-indigo-200 bg-indigo-50 p-3 text-xs text-indigo-900">
              Upgrade to Premium to unlock AI drafting, resume matching, and
              automated follow-ups. Billing checkout is coming soon.
            </div>
          )}
        </div>
      )}

      <div className="rounded border border-gray-200 bg-white p-4">
        <div className="text-sm font-medium mb-2">Backend health</div>
        {isLoading && <div className="text-gray-500 text-sm">Checking…</div>}
        {error && <div className="text-red-600 text-sm">Backend unreachable</div>}
        {data && (
          <pre className="text-xs text-gray-700">{JSON.stringify(data, null, 2)}</pre>
        )}
      </div>
    </section>
  );
}
