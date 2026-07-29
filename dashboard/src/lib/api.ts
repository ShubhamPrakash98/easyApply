import { createApiClient } from "@oneapply/api-client";

export const api = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080",
  getToken: () => localStorage.getItem("oneapply.jwt"),
});
