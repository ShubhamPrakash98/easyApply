import { createApiClient } from "@oneapply/api-client";

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

export const api = createApiClient({
  baseUrl: BASE_URL,
  getToken: async () => {
    const stored = await chrome.storage.local.get("jwt");
    return (stored.jwt as string | undefined) ?? null;
  },
});
