export interface ApiClientOptions {
  baseUrl: string;
  getToken?: () => string | null | Promise<string | null>;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public details?: unknown,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export function createApiClient(opts: ApiClientOptions) {
  const { baseUrl, getToken } = opts;

  async function request<T>(
    path: string,
    init: RequestInit = {},
  ): Promise<T> {
    const headers = new Headers(init.headers);
    // Don't set Content-Type for FormData — the browser needs to set it
    // with the multipart boundary.
    if (!headers.has("Content-Type") && init.body && !(init.body instanceof FormData)) {
      headers.set("Content-Type", "application/json");
    }
    if (getToken) {
      const token = await getToken();
      if (token) headers.set("Authorization", `Bearer ${token}`);
    }

    const res = await fetch(`${baseUrl}${path}`, {
      ...init,
      headers,
      credentials: "include",
    });

    if (!res.ok) {
      let payload: { code?: string; error?: string; message?: string; details?: unknown } = {};
      try {
        payload = await res.json();
      } catch {
        payload = { message: res.statusText };
      }
      throw new ApiError(
        res.status,
        payload.code ?? "unknown",
        payload.error ?? payload.message ?? res.statusText,
        payload.details,
      );
    }

    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  return {
    baseUrl,
    get: <T>(path: string) => request<T>(path, { method: "GET" }),
    post: <T>(path: string, body?: unknown) =>
      request<T>(path, {
        method: "POST",
        body: body ? JSON.stringify(body) : undefined,
      }),
    upload: <T>(path: string, formData: FormData) =>
      request<T>(path, { method: "POST", body: formData }),
    patch: <T>(path: string, body?: unknown) =>
      request<T>(path, {
        method: "PATCH",
        body: body ? JSON.stringify(body) : undefined,
      }),
    delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
  };
}

export type ApiClient = ReturnType<typeof createApiClient>;

// ---- Shared types (mirrors backend response shapes) ----

export interface HealthResponse {
  status: "ok";
  time: string;
  db?: "up" | "down";
}

export type SubscriptionTier = "free" | "premium";

/** Keys mirror internal/features/features.go */
export interface FeatureFlags {
  ai_draft_email: boolean;
  ai_resume_match: boolean;
  ai_followup: boolean;
}

export interface CurrentUser {
  id: string;
  email: string;
  name: string;
  trial_ends_at: string;
  gmail_connected: boolean;
  subscription_tier: SubscriptionTier;
  features: FeatureFlags;
}

export type OutreachStatus =
  | "pending_approval"
  | "sent"
  | "replied"
  | "followed_up"
  | "no_response"
  | "cancelled";

export interface CapturedProfile {
  recruiter_name: string;
  recruiter_headline: string;
  company: string;
  linkedin_url: string;
}

export interface DraftRequest extends CapturedProfile {
  job_description: string;
}

export interface DraftResponse {
  outreach_id: string;
  status: OutreachStatus;
  draft: { subject: string; body: string };
  contact: {
    id: string;
    name: string;
    email: string;
    company: string;
    source: string;
    verification_status: string;
  };
}

export interface OutreachListItem {
  id: string;
  status: OutreachStatus;
  subject: string;
  recruiter_name: string;
  recruiter_url: string;
  company: string;
  sent_at: string | null;
  created_at: string;
  follow_up_count: number;
}

export interface OutreachDetail {
  id: string;
  status: OutreachStatus;
  subject: string;
  body: string;
  job_description: string;
  created_at: string;
  sent_at: string | null;
  gmail_thread_id: string | null;
  follow_up_count: number;
  contact: {
    name: string;
    email: string;
    linkedin_url: string;
    source: string;
    verification_status: string;
  };
}

export interface ApproveResponse {
  id: string;
  status: OutreachStatus;
  gmail_thread_id: string | null;
  sent_at: string | null;
}

export interface ResumeSummary {
  id: string;
  label: string;
  created_at: string;
  extracted_text_len: number;
}
