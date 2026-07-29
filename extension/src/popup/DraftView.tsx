import { useState } from "react";
import { Button } from "@oneapply/ui";
import {
  ApiError,
  type ApproveResponse,
  type CapturedProfile,
  type DraftResponse,
} from "@oneapply/api-client";
import { api } from "../lib/api";

type Phase = "input" | "drafting" | "review" | "sending" | "sent" | "error";

interface Props {
  profile: CapturedProfile;
  onDone: () => void; // called after send/cancel to clear the pending capture
  onDismiss: () => void; // called to close the draft view without cancelling backend
}

export function DraftView({ profile, onDone, onDismiss }: Props) {
  const [phase, setPhase] = useState<Phase>("input");
  const [jd, setJd] = useState("");
  const [draft, setDraft] = useState<DraftResponse | null>(null);
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");
  const [error, setError] = useState<string | null>(null);

  async function handleDraft() {
    if (jd.trim().length < 10) {
      setError("Paste the job description first (at least 10 chars).");
      return;
    }
    setError(null);
    setPhase("drafting");
    try {
      const resp = await api.post<DraftResponse>("/api/outreach/draft", {
        ...profile,
        job_description: jd,
      });
      setDraft(resp);
      setSubject(resp.draft.subject);
      setBody(resp.draft.body);
      setPhase("review");
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : (err as Error).message;
      setError(msg);
      setPhase("error");
    }
  }

  async function handleApprove() {
    if (!draft) return;
    setError(null);
    setPhase("sending");
    try {
      await api.post<ApproveResponse>(
        `/api/outreach/${draft.outreach_id}/approve`,
        { subject, body },
      );
      setPhase("sent");
      onDone();
      setTimeout(onDismiss, 1500);
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : (err as Error).message;
      setError(msg);
      setPhase("error");
    }
  }

  async function handleCancel() {
    if (draft) {
      try {
        await api.post(`/api/outreach/${draft.outreach_id}/cancel`);
      } catch {
        // best-effort — server will auto-cancel pending drafts after 24h
      }
    }
    onDone();
    onDismiss();
  }

  return (
    <div className="space-y-3">
      <div className="rounded border border-gray-200 bg-white p-3 text-sm">
        <div className="font-medium text-gray-900">{profile.recruiter_name || "(no name)"}</div>
        {profile.recruiter_headline && (
          <div className="text-xs text-gray-600">{profile.recruiter_headline}</div>
        )}
        {profile.company && (
          <div className="mt-1 text-xs text-gray-500">at {profile.company}</div>
        )}
      </div>

      {(phase === "input" || phase === "drafting" || phase === "error") && (
        <>
          <label className="block text-xs font-medium text-gray-700">
            Job description
          </label>
          <textarea
            className="w-full rounded border border-gray-300 p-2 text-sm"
            rows={5}
            placeholder="Paste the JD you're reaching out about…"
            value={jd}
            onChange={(e) => setJd(e.target.value)}
            disabled={phase === "drafting"}
          />
          {error && (
            <div className="rounded border border-red-200 bg-red-50 p-2 text-xs text-red-700">
              {error}
            </div>
          )}
          <div className="flex gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={handleCancel}
              disabled={phase === "drafting"}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              className="flex-1"
              onClick={handleDraft}
              disabled={phase === "drafting"}
            >
              {phase === "drafting" ? "Drafting…" : "Draft email"}
            </Button>
          </div>
        </>
      )}

      {(phase === "review" || phase === "sending") && draft && (
        <>
          <div className="rounded border border-gray-200 bg-gray-50 p-2 text-xs text-gray-700">
            To <span className="font-medium">{draft.contact.email}</span>
            {" · "}
            <span title={draft.contact.verification_status}>
              {sourceLabel(draft.contact.source)}
            </span>
          </div>
          <label className="block text-xs font-medium text-gray-700">Subject</label>
          <input
            className="w-full rounded border border-gray-300 p-2 text-sm"
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            disabled={phase === "sending"}
          />
          <label className="block text-xs font-medium text-gray-700">Body</label>
          <textarea
            className="w-full rounded border border-gray-300 p-2 text-xs font-mono"
            rows={10}
            value={body}
            onChange={(e) => setBody(e.target.value)}
            disabled={phase === "sending"}
          />
          {error && (
            <div className="rounded border border-red-200 bg-red-50 p-2 text-xs text-red-700">
              {error}
            </div>
          )}
          <div className="flex gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={handleCancel}
              disabled={phase === "sending"}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              className="flex-1"
              onClick={handleApprove}
              disabled={phase === "sending"}
            >
              {phase === "sending" ? "Sending…" : "Approve & send"}
            </Button>
          </div>
        </>
      )}

      {phase === "sent" && (
        <div className="rounded border border-green-200 bg-green-50 p-3 text-sm text-green-800">
          Sent. Row is in your dashboard.
        </div>
      )}
    </div>
  );
}

function sourceLabel(source: string): string {
  switch (source) {
    case "cache":
      return "cached email";
    case "pattern":
      return "verified pattern";
    case "apollo":
      return "Apollo";
    case "stub":
      return "stub (Phase 2)";
    default:
      return source;
  }
}
