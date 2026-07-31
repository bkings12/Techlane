import { useEffect, useRef } from "react";
import { getAccessToken } from "./api";

const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080/api/v1";

export type RealtimeEvent = {
  event_type: string;
  occurred_at: string;
  branch_id?: string;
  payload: Record<string, unknown>;
};

type Listener = (event: RealtimeEvent) => void;

let source: EventSource | null = null;
let currentToken: string | null = null;
let retryTimer: number | null = null;
let retryDelayMs = 2_000;
const MAX_RETRY_DELAY_MS = 60_000;
const listeners = new Set<Listener>();

function scheduleReconnect() {
  if (retryTimer != null) return;
  retryTimer = window.setTimeout(() => {
    retryTimer = null;
    ensureConnection();
  }, retryDelayMs);
  retryDelayMs = Math.min(retryDelayMs * 2, MAX_RETRY_DELAY_MS);
}

function ensureConnection() {
  const token = getAccessToken();
  if (!token) return;
  if (source && currentToken === token) return;

  source?.close();
  currentToken = token;
  source = new EventSource(`${API_BASE}/events/stream?token=${encodeURIComponent(token)}`);
  source.onopen = () => {
    retryDelayMs = 2_000;
  };
  source.onmessage = (e) => {
    try {
      const parsed = JSON.parse(e.data) as RealtimeEvent;
      listeners.forEach((l) => l(parsed));
    } catch {
      // heartbeat / non-JSON comment lines are ignored
    }
  };
  source.onerror = () => {
    // A failed handshake (bad/expired token, server error) leaves EventSource
    // retrying on a fixed short interval — take over with our own backoff so a
    // broken stream can't hammer the API.
    source?.close();
    source = null;
    currentToken = null;
    scheduleReconnect();
  };
}

/**
 * Subscribe to live server events (repair status changes, part collection,
 * payment confirmations, ...). Matches by exact event_type or a "prefix.*"
 * pattern. Automatically connects/reconnects using the current session token.
 */
export function useRealtimeEvents(patterns: string[], onEvent: (event: RealtimeEvent) => void) {
  const handlerRef = useRef(onEvent);
  handlerRef.current = onEvent;

  useEffect(() => {
    ensureConnection();
    const listener: Listener = (event) => {
      const matches = patterns.some((p) => {
        if (p === "*") return true;
        if (p.endsWith(".*")) return event.event_type.startsWith(p.slice(0, -1));
        return event.event_type === p;
      });
      if (matches) handlerRef.current(event);
    };
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [patterns.join(",")]);
}

export function closeRealtimeConnection() {
  if (retryTimer != null) {
    window.clearTimeout(retryTimer);
    retryTimer = null;
  }
  retryDelayMs = 2_000;
  source?.close();
  source = null;
  currentToken = null;
}
