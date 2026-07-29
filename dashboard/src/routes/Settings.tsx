import { useQuery } from "@tanstack/react-query";
import type { HealthResponse } from "@oneapply/api-client";
import { api } from "../lib/api";

export function Settings() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["health"],
    queryFn: () => api.get<HealthResponse>("/health"),
  });

  return (
    <section>
      <h1 className="text-2xl font-semibold mb-2">Settings</h1>
      <p className="text-gray-600 mb-6">Gmail connection, quota, cadence. Coming in Phase 5.</p>

      <div className="rounded border border-gray-200 bg-white p-4 max-w-md">
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
