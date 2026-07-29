import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import type { OutreachDetail } from "@oneapply/api-client";
import { api } from "../lib/api";

interface Props {
  id: string;
  onClose: () => void;
}

export function OutreachDrawer({ id, onClose }: Props) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["outreach", "detail", id],
    queryFn: () => api.get<OutreachDetail>(`/api/outreach/${id}`),
  });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-40" onClick={onClose}>
      <div className="absolute inset-0 bg-black/30" />
      <aside
        className="absolute right-0 top-0 h-full w-full max-w-xl overflow-auto bg-white shadow-xl p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between mb-4">
          <h2 className="text-lg font-semibold">Outreach detail</h2>
          <button
            onClick={onClose}
            className="text-gray-500 hover:text-gray-900 text-sm"
          >
            Close ✕
          </button>
        </div>

        {isLoading && <div className="text-sm text-gray-500">Loading…</div>}
        {error && (
          <div className="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {(error as Error).message}
          </div>
        )}

        {data && (
          <div className="space-y-5">
            <div>
              <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">Recruiter</div>
              <div className="text-sm">
                <div className="font-medium text-gray-900">{data.contact.name}</div>
                <div className="text-gray-600">{data.contact.email}</div>
                {data.contact.linkedin_url && (
                  <a
                    href={data.contact.linkedin_url}
                    target="_blank"
                    rel="noreferrer"
                    className="text-xs text-indigo-600 hover:underline"
                  >
                    {data.contact.linkedin_url}
                  </a>
                )}
                <div className="mt-1 text-xs text-gray-500">
                  source: {data.contact.source} · verification: {data.contact.verification_status}
                </div>
              </div>
            </div>

            <div>
              <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">Status</div>
              <div className="text-sm text-gray-900">{data.status.replace(/_/g, " ")}</div>
              {data.sent_at && (
                <div className="text-xs text-gray-500">
                  sent {new Date(data.sent_at).toLocaleString()}
                </div>
              )}
              {data.gmail_thread_id && (
                <div className="text-xs text-gray-500 font-mono">thread: {data.gmail_thread_id}</div>
              )}
              {data.follow_up_count > 0 && (
                <div className="text-xs text-gray-500">follow-ups: {data.follow_up_count}</div>
              )}
            </div>

            <div>
              <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">Subject</div>
              <div className="text-sm text-gray-900">{data.subject}</div>
            </div>

            <div>
              <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">Body</div>
              <pre className="whitespace-pre-wrap text-xs text-gray-800 bg-gray-50 border border-gray-200 rounded p-3 font-mono">
                {data.body}
              </pre>
            </div>

            <div>
              <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">Job description</div>
              <pre className="whitespace-pre-wrap text-xs text-gray-800 bg-gray-50 border border-gray-200 rounded p-3">
                {data.job_description}
              </pre>
            </div>
          </div>
        )}
      </aside>
    </div>
  );
}
