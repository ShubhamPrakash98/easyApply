import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@oneapply/ui";
import { ApiError, type ResumeSummary } from "@oneapply/api-client";
import { api } from "../lib/api";

export function Resumes() {
  const qc = useQueryClient();
  const [label, setLabel] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["resumes"],
    queryFn: () =>
      api.get<{ items: ResumeSummary[] }>("/api/resumes").then((r) => r.items),
  });

  const upload = useMutation({
    mutationFn: async () => {
      if (!file) throw new Error("Pick a PDF first");
      if (!label.trim()) throw new Error("Label is required");
      const fd = new FormData();
      fd.append("label", label.trim());
      fd.append("file", file);
      return api.upload<ResumeSummary>("/api/resumes", fd);
    },
    onSuccess: () => {
      setLabel("");
      setFile(null);
      if (fileInputRef.current) fileInputRef.current.value = "";
      setError(null);
      qc.invalidateQueries({ queryKey: ["resumes"] });
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : (err as Error).message);
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.delete<void>(`/api/resumes/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["resumes"] }),
  });

  return (
    <section className="max-w-3xl">
      <h1 className="text-2xl font-semibold mb-2">Resumes</h1>
      <p className="text-gray-600 mb-6 text-sm">
        Upload 2–3 variants labelled by role type. The AI picks the best match per JD when drafting.
      </p>

      <div className="rounded border border-gray-200 bg-white p-4 space-y-3 mb-8">
        <div className="text-sm font-medium">Upload a resume</div>
        <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto] gap-3">
          <input
            className="rounded border border-gray-300 px-3 py-2 text-sm"
            placeholder="Label (e.g. Backend Go, Frontend React)"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
          />
          <input
            ref={fileInputRef}
            type="file"
            accept="application/pdf,.pdf"
            className="text-sm"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </div>
        {error && (
          <div className="rounded border border-red-200 bg-red-50 p-2 text-xs text-red-700">
            {error}
          </div>
        )}
        <div>
          <Button
            variant="primary"
            size="sm"
            onClick={() => upload.mutate()}
            disabled={upload.isPending || !file || !label.trim()}
          >
            {upload.isPending ? "Uploading…" : "Upload PDF"}
          </Button>
        </div>
      </div>

      {isLoading && <div className="text-sm text-gray-500">Loading…</div>}

      {data && data.length === 0 && (
        <div className="rounded border border-dashed border-gray-300 bg-white p-8 text-center text-sm text-gray-600">
          No resumes yet. Upload your first one above.
        </div>
      )}

      {data && data.length > 0 && (
        <div className="overflow-hidden rounded border border-gray-200 bg-white">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 text-xs uppercase text-gray-500">
              <tr>
                <th className="px-4 py-2 text-left">Label</th>
                <th className="px-4 py-2 text-left">Uploaded</th>
                <th className="px-4 py-2 text-left">Extracted text</th>
                <th className="px-4 py-2 text-right"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {data.map((r) => (
                <tr key={r.id}>
                  <td className="px-4 py-3 font-medium text-gray-900">{r.label}</td>
                  <td className="px-4 py-3 text-gray-600">
                    {new Date(r.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 text-xs text-gray-500">
                    {r.extracted_text_len > 0
                      ? `${r.extracted_text_len.toLocaleString()} chars`
                      : "no text (image-only PDF)"}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => {
                        if (confirm(`Delete "${r.label}"?`)) remove.mutate(r.id);
                      }}
                      className="text-xs text-red-600 hover:text-red-800"
                      disabled={remove.isPending}
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
