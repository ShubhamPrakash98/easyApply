import { useEffect, useState } from "react";
import { Button } from "@oneapply/ui";
import {
  ApiError,
  type ApproveResponse,
  type CapturedProfile,
  type CurrentUser,
  type DraftResponse,
  type ResumeSummary,
} from "@oneapply/api-client";
import { api } from "../lib/api";

type Phase = "input" | "preparing" | "review" | "sending" | "sent" | "error";

interface Props {
  profile: CapturedProfile;
  user: CurrentUser;
  onDone: () => void; // clear the pending capture
  onDismiss: () => void; // close the draft view
}

export function DraftView({ profile, user, onDone, onDismiss }: Props) {
  const aiDraft = user.features.ai_draft_email;

  const [phase, setPhase] = useState<Phase>("input");
  const [jd, setJd] = useState("");
  const [resumes, setResumes] = useState<ResumeSummary[]>([]);
  const [resumeID, setResumeID] = useState<string>("");
  const [draft, setDraft] = useState<DraftResponse | null>(null);
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void api
      .get<{ items: ResumeSummary[] }>("/api/resumes")
      .then((r) => setResumes(r.items))
      .catch(() => setResumes([]));
  }, []);

  async function handlePrepare() {
    if (jd.trim().length < 10) {
      setError("Paste the job description first (at least 10 chars).");
      return;
    }
    setError(null);
    setPhase("preparing");
    try {
      const resp = await api.post<DraftResponse>("/api/outreach/draft", {
        ...profile,
        job_description: jd,
        resume_id: resumeID || undefined,
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
    if (!subject.trim() || !body.trim()) {
      setError("Subject and body are required before sending.");
      return;
    }
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
        // best-effort
      }
    }
    onDone();
    onDismiss();
  }

  const prepareLabel = aiDraft ? "Draft email" : "Find email & continue";
  const preparingLabel = aiDraft ? "Drafting…" : "Finding email…";

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

      {(phase === "input" || phase === "preparing" || phase === "error") && (
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
            disabled={phase === "preparing"}
          />

          <label className="block text-xs font-medium text-gray-700">
            Resume
          </label>
          <select
            className="w-full rounded border border-gray-300 p-2 text-sm bg-white"
            value={resumeID}
            onChange={(e) => setResumeID(e.target.value)}
            disabled={phase === "preparing"}
          >
            <option value="">
              {user.features.ai_resume_match ? "Let AI pick the best match" : "Most recent resume"}
            </option>
            {resumes.map((r) => (
              <option key={r.id} value={r.id}>{r.label}</option>
            ))}
          </select>
          {resumes.length === 0 && (
            <p className="text-xs text-gray-500">
              No resumes uploaded yet. Upload from the dashboard so we can attach one.
            </p>
          )}

          {!aiDraft && (
            <div className="rounded border border-indigo-200 bg-indigo-50 p-2 text-xs text-indigo-900">
              You'll write the email yourself on the next screen.
              <br />
              <span className="text-indigo-700">AI-drafted emails are a Premium feature — coming soon.</span>
            </div>
          )}

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
              disabled={phase === "preparing"}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              className="flex-1"
              onClick={handlePrepare}
              disabled={phase === "preparing"}
            >
              {phase === "preparing" ? preparingLabel : prepareLabel}
            </Button>
          </div>
        </>
      )}

      {(phase === "review" || phase === "sending") && draft && (
        <>
          <div className="rounded border border-gray-200 bg-gray-50 p-2 text-xs text-gray-700 space-y-1">
            <div>To <span className="font-medium">{draft.contact.email}</span></div>
            <div className="flex items-center gap-2">
              <VerificationBadge status={draft.contact.verification_status} />
              <span className="text-gray-500">source: {sourceLabel(draft.contact.source)}</span>
            </div>
          </div>

          {!aiDraft && (
            <div className="rounded border border-gray-200 bg-gray-50 p-2 text-xs text-gray-700">
              <span className="font-medium">Reference JD:</span>
              <div className="mt-1 max-h-24 overflow-auto whitespace-pre-wrap text-gray-600">
                {jd}
              </div>
            </div>
          )}

          <label className="block text-xs font-medium text-gray-700">Subject</label>
          <input
            className="w-full rounded border border-gray-300 p-2 text-sm"
            value={subject}
            placeholder={aiDraft ? "" : "Write a subject line"}
            onChange={(e) => setSubject(e.target.value)}
            disabled={phase === "sending"}
          />
          <label className="block text-xs font-medium text-gray-700">Body</label>
          <textarea
            className="w-full rounded border border-gray-300 p-2 text-xs font-mono"
            rows={12}
            value={body}
            placeholder={aiDraft ? "" : "Hi [name],\n\nWrite your email here…"}
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
              disabled={phase === "sending" || !subject.trim() || !body.trim()}
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

function VerificationBadge({ status }: { status: string }) {
  const map: Record<string, { label: string; cls: string }> = {
    deliverable: { label: "verified", cls: "bg-green-100 text-green-800" },
    risky: { label: "risky (catch-all)", cls: "bg-yellow-100 text-yellow-800" },
    invalid: { label: "invalid", cls: "bg-red-100 text-red-800" },
    unknown: { label: "unverified", cls: "bg-gray-100 text-gray-700" },
  };
  const conf = map[status] ?? map.unknown;
  return (
    <span className={`inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium ${conf.cls}`}>
      {conf.label}
    </span>
  );
}

function sourceLabel(source: string): string {
  switch (source) {
    case "cache":
      return "cached";
    case "pattern":
      return "verified pattern";
    case "apollo":
      return "Apollo";
    case "stub":
      return "stub";
    default:
      return source;
  }
}
