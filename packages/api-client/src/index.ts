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
    if (!headers.has("Content-Type") && init.body) {
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
    patch: <T>(path: string, body?: unknown) =>
      request<T>(path, {
        method: "PATCH",
        body: body ? JSON.stringify(body) : undefined,
      }),
    delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
  };
}

export type ApiClient = ReturnType<typeof createApiClient>;

// Shared types — expand as endpoints are added in later phases.
export interface HealthResponse {
  status: "ok";
  time: string;
  db?: "up" | "down";
}

export interface CurrentUser {
  id: string;
  email: string;
  name: string;
  trial_ends_at: string;
  gmail_connected: boolean;
}
