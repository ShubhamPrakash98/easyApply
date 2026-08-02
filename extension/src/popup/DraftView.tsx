import { useEffect, useState } from "react";
import { Button } from "@oneapply/ui";
import {
  ApiError,
  type ApproveResponse,
  type CapturedProfile,
  type ContactSummary,
  type CurrentUser,
  type DraftResponse,
  type FindEmailResponse,
  type ResumeSummary,
} from "@oneapply/api-client";
import { api } from "../lib/api";

type Phase =
  | "await_find"      // step 1: shows Find email button
  | "finding"         // step 1 in-flight
  | "await_draft"     // step 2: email found, JD input + resume picker
  | "preparing"       // step 2 in-flight (LLM if premium)
  | "review"          // step 3: editable subject/body
  | "sending"         // approve in-flight
  | "sent"
  | "error";

interface Props {
  profile: CapturedProfile;
  user: CurrentUser;
  onDone: () => void;
  onDismiss: () => void;
}

export function DraftView({ profile, user, onDone, onDismiss }: Props) {
  const aiDraft = user.features.ai_draft_email;

  const [phase, setPhase] = useState<Phase>("await_find");
  const [error, setError] = useState<string | null>(null);

  // Step 1 output
  const [contact, setContact] = useState<ContactSummary | null>(null);

  // Step 2 inputs
  const [jd, setJd] = useState("");
  const [resumes, setResumes] = useState<ResumeSummary[]>([]);
  const [resumeID, setResumeID] = useState<string>("");

  // Step 3 (review) state
  const [draft, setDraft] = useState<DraftResponse | null>(null);
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");

  useEffect(() => {
    void api
      .get<{ items: ResumeSummary[] }>("/api/resumes")
      .then((r) => setResumes(r.items))
      .catch(() => setResumes([]));
  }, []);

  async function handleFindEmail() {
    setError(null);
    setPhase("finding");
    try {
      const resp = await api.post<FindEmailResponse>("/api/outreach/find-email", {
        recruiter_name: profile.recruiter_name,
        company: profile.company,
        linkedin_url: profile.linkedin_url,
      });
      setContact(resp.contact);
      setPhase("await_draft");
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : (err as Error).message;
      setError(msg);
      setPhase("error");
    }
  }

  async function handleDraft() {
    if (!contact) return;
    if (jd.trim().length < 10) {
      setError("Paste the job description first (at least 10 chars).");
      return;
    }
    setError(null);
    setPhase("preparing");
    try {
      const resp = await api.post<DraftResponse>("/api/outreach/draft", {
        contact_id: contact.id,
        recruiter_headline: profile.recruiter_headline,
        company: profile.company,
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

  return (
    <div className="space-y-3">
      {/* recruiter header always visible */}
      <div className="rounded border border-gray-200 bg-white p-3 text-sm">
        <div className="font-medium text-gray-900">{profile.recruiter_name || "(no name)"}</div>
        {profile.recruiter_headline && (
          <div className="text-xs text-gray-600">{profile.recruiter_headline}</div>
        )}
        {profile.company && (
          <div className="mt-1 text-xs text-gray-500">at {profile.company}</div>
        )}
      </div>

      {/* found email chip — appears after step 1 succeeds */}
      {contact && phase !== "sent" && (
        <div className="rounded border border-gray-200 bg-gray-50 p-2 text-xs text-gray-700 space-y-1">
          <div className="flex items-center gap-2">
            <span className="text-green-700">✓</span>
            <span className="font-medium">{contact.email}</span>
          </div>
          <div className="flex items-center gap-2">
            <VerificationBadge status={contact.verification_status} />
            <span className="text-gray-500">source: {sourceLabel(contact.source)}</span>
          </div>
        </div>
      )}

      {/* PHASE 1 — Find email */}
      {(phase === "await_find" || phase === "finding") && (
        <>
          <div className="rounded border border-indigo-200 bg-indigo-50 p-3 text-sm text-indigo-900">
            Step 1 of 2 — look up this recruiter's email using the pattern +
            SMTP-verify cascade. Free, no send yet.
          </div>
          <div className="flex gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={handleCancel}
              disabled={phase === "finding"}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              className="flex-1"
              onClick={handleFindEmail}
              disabled={phase === "finding"}
            >
              {phase === "finding" ? "Searching…" : "Find email"}
            </Button>
          </div>
          {error && (
            <div className="rounded border border-red-200 bg-red-50 p-2 text-xs text-red-700">
              {error}
            </div>
          )}
        </>
      )}

      {/* PHASE 2 — JD + resume + draft */}
      {(phase === "await_draft" || phase === "preparing") && (
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

          <label className="block text-xs font-medium text-gray-700">Resume</label>
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
              <span className="text-indigo-700"> AI drafts are a Premium feature.</span>
            </div>
          )}

          {error && (
            <div className="rounded border border-red-200 bg-red-50 p-2 text-xs text-red-700">
              {error}
            </div>
          )}
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" onClick={handleCancel} disabled={phase === "preparing"}>
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              className="flex-1"
              onClick={handleDraft}
              disabled={phase === "preparing"}
            >
              {phase === "preparing"
                ? aiDraft ? "Drafting…" : "Preparing…"
                : aiDraft ? "Draft email" : "Continue"}
            </Button>
          </div>
        </>
      )}

      {/* PHASE 3 — review + approve */}
      {(phase === "review" || phase === "sending") && draft && (
        <>
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
            <Button variant="secondary" size="sm" onClick={handleCancel} disabled={phase === "sending"}>
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

      {phase === "error" && !contact && (
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={handleCancel}>
            Cancel
          </Button>
          <Button variant="primary" size="sm" className="flex-1" onClick={() => setPhase("await_find")}>
            Try again
          </Button>
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
