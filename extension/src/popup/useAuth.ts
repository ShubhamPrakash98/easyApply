import { useEffect, useRef, useState } from "react";
import { ApiError, type CurrentUser } from "@oneapply/api-client";
import { api } from "../lib/api";

type Status = "loading" | "authenticated" | "unauthenticated" | "error";

interface State {
  status: Status;
  user: CurrentUser | null;
  error: string | null;
}

const initial: State = { status: "loading", user: null, error: null };

// While unauthenticated we poll /me so that once the user completes the OAuth
// flow in the dashboard tab, the popup picks it up without a manual reload.
const POLL_MS = 3000;

export function useAuth() {
  const [state, setState] = useState<State>(initial);
  const timerRef = useRef<number | null>(null);

  const refetch = async () => {
    try {
      const user = await api.get<CurrentUser>("/api/auth/me");
      setState({ status: "authenticated", user, error: null });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setState({ status: "unauthenticated", user: null, error: null });
      } else {
        setState({ status: "error", user: null, error: (err as Error).message });
      }
    }
  };

  useEffect(() => {
    void refetch();
  }, []);

  useEffect(() => {
    if (state.status !== "unauthenticated") {
      if (timerRef.current) window.clearInterval(timerRef.current);
      timerRef.current = null;
      return;
    }
    timerRef.current = window.setInterval(refetch, POLL_MS);
    return () => {
      if (timerRef.current) window.clearInterval(timerRef.current);
    };
  }, [state.status]);

  const signIn = () => {
    const url = `${(import.meta.env.VITE_DASHBOARD_URL as string | undefined) ?? "http://localhost:5173"}/login`;
    chrome.tabs.create({ url });
  };

  const signOut = async () => {
    try {
      await api.post<void>("/api/auth/logout");
    } finally {
      setState({ status: "unauthenticated", user: null, error: null });
    }
  };

  return { ...state, signIn, signOut, refetch };
}
