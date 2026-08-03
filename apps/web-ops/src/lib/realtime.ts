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
let pollTimer: number | null = null;
let retryDelayMs = 2_000;
let consecutiveFailures = 0;
/**
 * Cloudflare advertises HTTP/3 (Alt-Svc: h3) and long-lived EventSource streams
 * die with net::ERR_QUIC_PROTOCOL_ERROR. Skip SSE on our CF-fronted hosts and
 * use quiet polling instead — no console spam, still near-live updates.
 */
let preferPoll =
  typeof window !== "undefined" && /\.techlane\.co\.ke$/i.test(window.location.hostname);
const MAX_RETRY_DELAY_MS = 60_000;
const POLL_EVERY_MS = 12_000;
const listeners = new Set<Listener>();

const POLL_EVENT: RealtimeEvent = {
  event_type: "__poll__",
  occurred_at: "",
  payload: {},
};

function clearRetry() {
  if (retryTimer != null) {
    window.clearTimeout(retryTimer);
    retryTimer = null;
  }
}

function clearPoll() {
  if (pollTimer != null) {
    window.clearInterval(pollTimer);
    pollTimer = null;
  }
}

function emitPoll() {
  POLL_EVENT.occurred_at = new Date().toISOString();
  listeners.forEach((l) => l(POLL_EVENT));
}

function startPollFallback() {
  if (pollTimer != null) return;
  preferPoll = true;
  source?.close();
  source = null;
  currentToken = null;
  clearRetry();
  emitPoll();
  pollTimer = window.setInterval(emitPoll, POLL_EVERY_MS);
}

function scheduleReconnect() {
  if (preferPoll || retryTimer != null) return;
  retryTimer = window.setTimeout(() => {
    retryTimer = null;
    ensureConnection();
  }, retryDelayMs);
  retryDelayMs = Math.min(retryDelayMs * 2, MAX_RETRY_DELAY_MS);
}

function streamBase() {
  // Same-origin /api when on *.techlane.co.ke (Caddy proxies to platform).
  if (typeof window !== "undefined" && /\.techlane\.co\.ke$/i.test(window.location.hostname)) {
    return `${window.location.origin}/api/v1`;
  }
  return API_BASE;
}

function ensureConnection() {
  const token = getAccessToken();
  if (!token) return;

  if (preferPoll) {
    startPollFallback();
    return;
  }

  if (source && currentToken === token) return;

  source?.close();
  currentToken = token;
  source = new EventSource(`${streamBase()}/events/stream?token=${encodeURIComponent(token)}`);
  source.onopen = () => {
    consecutiveFailures = 0;
    retryDelayMs = 2_000;
  };
  source.onmessage = (e) => {
    consecutiveFailures = 0;
    try {
      const parsed = JSON.parse(e.data) as RealtimeEvent;
      listeners.forEach((l) => l(parsed));
    } catch {
      // heartbeat / non-JSON comment lines are ignored
    }
  };
  source.onerror = () => {
    // EventSource surfaces Cloudflare QUIC drops as errors after a 200 — stop
    // hammering the stream and fall back to quiet polling.
    source?.close();
    source = null;
    currentToken = null;
    consecutiveFailures += 1;
    if (consecutiveFailures >= 2) {
      startPollFallback();
      return;
    }
    scheduleReconnect();
  };
}

/**
 * Subscribe to live server events (repair status changes, part collection,
 * payment confirmations, ...). Matches by exact event_type or a "prefix.*"
 * pattern. Automatically connects/reconnects using the current session token.
 * Behind Cloudflare HTTP/3, falls back to a quiet poll so the console is not
 * flooded with net::ERR_QUIC_PROTOCOL_ERROR.
 */
export function useRealtimeEvents(patterns: string[], onEvent: (event: RealtimeEvent) => void) {
  const handlerRef = useRef(onEvent);
  handlerRef.current = onEvent;

  useEffect(() => {
    ensureConnection();
    const listener: Listener = (event) => {
      if (event.event_type === "__poll__") {
        handlerRef.current(event);
        return;
      }
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
  clearRetry();
  clearPoll();
  preferPoll = false;
  consecutiveFailures = 0;
  retryDelayMs = 2_000;
  source?.close();
  source = null;
  currentToken = null;
}
