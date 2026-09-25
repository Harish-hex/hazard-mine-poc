"use client";

import { useEffect, useRef, useState } from "react";
import type { DashboardState } from "./types";

export interface LogEntry {
  id: string;
  ts: string;
  text: string;
}

let seq = 0;
function nextId(): string {
  seq += 1;
  return `log-${seq}`;
}

// The backend has no dedicated event-log endpoint. Entries are derived
// client-side by diffing each incoming DashboardState against the previous
// one. The very first snapshot only establishes the baseline (no entries
// logged for it) so the log doesn't spam on initial connect.
export function useEventLog(
  state: DashboardState | null,
  nameById: Map<string, string>
): LogEntry[] {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const prevRef = useRef<DashboardState | null>(null);

  useEffect(() => {
    if (!state) return;
    const prev = prevRef.current;
    prevRef.current = state;

    if (!prev) return;

    const additions: string[] = [];

    if (prev.stall.is_stalled !== state.stall.is_stalled) {
      additions.push(state.stall.is_stalled ? "STALL DETECTED" : "stall cleared");
    }

    const prevEdgesKey = prev.route.edges.join(",");
    const nextEdgesKey = state.route.edges.join(",");
    if (prevEdgesKey !== nextEdgesKey) {
      const label = state.route.nodes
        .map((id) => nameById.get(id) ?? id)
        .join(" → ");
      additions.push(`route recalculated: ${label || "—"}`);
    }

    const prevBlocked = new Set(prev.blocked_edges);
    const nextBlocked = new Set(state.blocked_edges);
    for (const edgeId of nextBlocked) {
      if (!prevBlocked.has(edgeId)) additions.push(`edge ${edgeId} blocked`);
    }
    for (const edgeId of prevBlocked) {
      if (!nextBlocked.has(edgeId)) additions.push(`edge ${edgeId} cleared`);
    }

    if (!prev.arrived && state.arrived) {
      additions.push("arrived at EXIT");
    }

    if (additions.length > 0) {
      const ts = new Date(state.timestamp).toLocaleTimeString();
      // This effect's entire job is diffing two WS snapshots (external
      // system) and appending derived log entries; there's no pure-render
      // form of "diff against the previous push."
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setEntries((cur) => [
        ...additions.map((text) => ({ id: nextId(), ts, text })).reverse(),
        ...cur,
      ]);
    }
  }, [state, nameById]);

  return entries;
}
