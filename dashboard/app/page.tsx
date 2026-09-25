"use client";

import { useEffect, useMemo, useState } from "react";
import { fetchGraph, fetchState } from "@/lib/api";
import type { DashboardState, GraphResponse } from "@/lib/types";
import { useDashboardSocket } from "@/lib/useDashboardSocket";
import { useEventLog } from "@/lib/useEventLog";
import Minimap from "@/components/Minimap";
import RouteStrip from "@/components/RouteStrip";
import StatusPanel from "@/components/StatusPanel";
import EventLog from "@/components/EventLog";
import OverridePanel from "@/components/OverridePanel";

export default function Home() {
  const [graph, setGraph] = useState<GraphResponse | null>(null);
  const [graphError, setGraphError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchGraph()
      .then((g) => {
        if (!cancelled) setGraph(g);
      })
      .catch((err) => {
        if (!cancelled) {
          setGraphError(err instanceof Error ? err.message : String(err));
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const { state: wsState, status } = useDashboardSocket();

  // The dashboard WS only pushes on the NEXT broadcast (telemetry tick,
  // hazard, override) — a fresh subscriber gets nothing until then, which
  // would otherwise leave the whole dashboard blank for however long it
  // takes the vehicle/mock client to send its first sample. Seed from the
  // REST snapshot every time the socket (re)connects so there's always
  // something to render immediately; the first live WS push after that
  // takes over until the next drop. This also covers backend-restart
  // recovery (in-memory state, resets on restart per the project spec):
  // useDashboardSocket clears its held state on disconnect, so without this
  // re-fetch the dashboard would otherwise sit blank after a reconnect until
  // the backend's next incidental broadcast.
  const [restState, setRestState] = useState<DashboardState | null>(null);
  useEffect(() => {
    if (status !== "connected") return;
    let cancelled = false;
    fetchState()
      .then((s) => {
        if (!cancelled) setRestState(s);
      })
      .catch(() => {
        // Non-fatal: the dashboard just stays blank until the first WS
        // push, same as before this fallback existed.
      });
    return () => {
      cancelled = true;
    };
  }, [status]);

  const state = wsState ?? restState;

  const nameById = useMemo(
    () => new Map((graph?.nodes ?? []).map((n) => [n.id, n.name])),
    [graph]
  );

  const logEntries = useEventLog(state, nameById);

  return (
    <div className="flex h-full w-full gap-3 p-3">
      <div className="flex min-w-0 flex-[7] flex-col gap-3">
        <div className="min-h-0 flex-1">
          <Minimap graph={graph} state={state} />
        </div>
        <div className="h-12 shrink-0">
          <RouteStrip graph={graph} state={state} />
        </div>
      </div>

      <div className="flex min-w-0 flex-[3] flex-col gap-3">
        <div className="shrink-0">
          <StatusPanel state={state} wsStatus={status} />
        </div>
        <EventLog entries={logEntries} />
        <div className="shrink-0">
          <OverridePanel graph={graph} state={state} />
        </div>
      </div>

      {graphError && (
        <div className="fixed bottom-3 left-3 rounded bg-red-950 px-3 py-2 text-xs text-red-200">
          Failed to load graph topology: {graphError}
        </div>
      )}
    </div>
  );
}
