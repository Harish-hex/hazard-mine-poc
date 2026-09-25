"use client";

import { useEffect, useRef, useState } from "react";
import type { DashboardState } from "./types";

export type WsConnectionStatus = "connecting" | "connected" | "reconnecting";

const WS_URL =
  process.env.NEXT_PUBLIC_BACKEND_WS_URL ?? "ws://localhost:8080/ws/dashboard";

// Reconnect backoff: 1s, 2s, then capped at 5s. Retries forever — a dropped
// connection mid-pitch is the worst failure mode, so we never give up.
const BACKOFF_STEPS_MS = [1000, 2000, 5000];

export function useDashboardSocket() {
  const [state, setState] = useState<DashboardState | null>(null);
  const [status, setStatus] = useState<WsConnectionStatus>("connecting");

  const attemptRef = useRef(0);
  const wsRef = useRef<WebSocket | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const unmountedRef = useRef(false);

  useEffect(() => {
    unmountedRef.current = false;

    function scheduleReconnect() {
      if (unmountedRef.current) return;
      const idx = Math.min(attemptRef.current, BACKOFF_STEPS_MS.length - 1);
      const delay = BACKOFF_STEPS_MS[idx];
      attemptRef.current += 1;
      timerRef.current = setTimeout(connect, delay);
    }

    function connect() {
      if (unmountedRef.current) return;
      setStatus(attemptRef.current === 0 ? "connecting" : "reconnecting");

      let ws: WebSocket;
      try {
        ws = new WebSocket(WS_URL);
      } catch {
        scheduleReconnect();
        return;
      }
      wsRef.current = ws;

      ws.onopen = () => {
        attemptRef.current = 0;
        setStatus("connected");
      };

      ws.onmessage = (ev) => {
        try {
          const parsed = JSON.parse(ev.data) as DashboardState;
          setState(parsed);
        } catch {
          // ignore malformed frame
        }
      };

      ws.onclose = () => {
        if (unmountedRef.current) return;
        setStatus("reconnecting");
        // A dropped connection means the held snapshot is no longer backed
        // by a live guarantee — the backend may have restarted (in-memory
        // state, resets on restart per the project spec) and could resume
        // at a totally different cart position. Clear it so callers fall
        // back to a fresh REST re-fetch on reconnect instead of silently
        // showing stale pre-drop data indefinitely.
        setState(null);
        scheduleReconnect();
      };

      ws.onerror = () => {
        ws.close();
      };
    }

    connect();

    return () => {
      unmountedRef.current = true;
      if (timerRef.current) clearTimeout(timerRef.current);
      wsRef.current?.close();
    };
  }, []);

  return { state, status };
}
