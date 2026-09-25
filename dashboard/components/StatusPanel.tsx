"use client";

import { useEffect, useRef, useState } from "react";
import type { DashboardState } from "@/lib/types";
import type { WsConnectionStatus } from "@/lib/useDashboardSocket";

interface StatusPanelProps {
  state: DashboardState | null;
  wsStatus: WsConnectionStatus;
}

const ESP32_STALE_AFTER_MS = 1500;

function Dot({
  color,
  pulse,
}: {
  color: "green" | "gray" | "amber" | "red";
  pulse: boolean;
}) {
  const base: Record<string, string> = {
    green: "bg-lime-400",
    gray: "bg-neutral-600",
    amber: "bg-amber-400",
    red: "bg-red-500",
  };
  return (
    <span className="relative inline-flex h-2.5 w-2.5">
      {pulse && (
        <span
          className={`absolute inline-flex h-full w-full animate-ping rounded-full ${base[color]} opacity-75`}
        />
      )}
      <span
        className={`relative inline-flex h-2.5 w-2.5 rounded-full ${base[color]}`}
      />
    </span>
  );
}

export default function StatusPanel({ state, wsStatus }: StatusPanelProps) {
  const [esp32Live, setEsp32Live] = useState(false);
  const [esp32Pulse, setEsp32Pulse] = useState(false);
  const [phonePulse, setPhonePulse] = useState(false);

  const prevLastUpdateTs = useRef<number | null>(null);
  const prevHazardAt = useRef<string | null>(null);
  const esp32PulseTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const phonePulseTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // ESP32 link freshness: checked on an interval, not just on message
  // receipt, so it correctly goes stale if telemetry stops arriving.
  useEffect(() => {
    const tick = () => {
      const lastTs = state?.telemetry.last_update_ts;
      if (!lastTs) {
        setEsp32Live(false);
        return;
      }
      setEsp32Live(Date.now() - lastTs < ESP32_STALE_AFTER_MS);
    };
    tick();
    const id = setInterval(tick, 300);
    return () => clearInterval(id);
  }, [state?.telemetry.last_update_ts]);

  // Pulse the ESP32 dot whenever last_update_ts actually changes.
  useEffect(() => {
    const lastTs = state?.telemetry.last_update_ts;
    if (lastTs === undefined) return;
    if (prevLastUpdateTs.current !== null && prevLastUpdateTs.current !== lastTs) {
      setEsp32Pulse(true);
      if (esp32PulseTimer.current) clearTimeout(esp32PulseTimer.current);
      esp32PulseTimer.current = setTimeout(() => setEsp32Pulse(false), 350);
    }
    prevLastUpdateTs.current = lastTs;
  }, [state?.telemetry.last_update_ts]);

  // Flash the phone dot whenever a newer phone-sourced hazard arrives.
  useEffect(() => {
    const hazard = state?.last_hazard;
    if (!hazard) return;
    if (prevHazardAt.current !== hazard.at && hazard.source === "phone") {
      setPhonePulse(true);
      if (phonePulseTimer.current) clearTimeout(phonePulseTimer.current);
      phonePulseTimer.current = setTimeout(() => setPhonePulse(false), 2000);
    }
    prevHazardAt.current = hazard.at;
  }, [state?.last_hazard]);

  const wsLabel: Record<WsConnectionStatus, string> = {
    connecting: "Connecting…",
    connected: "Connected",
    reconnecting: "Reconnecting…",
  };
  const wsColor: Record<WsConnectionStatus, "green" | "amber" | "gray"> = {
    connecting: "amber",
    connected: "green",
    reconnecting: "amber",
  };

  const stall = state?.stall;
  const edge = state?.current_edge;

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-neutral-800 bg-[#171310] p-4">
      <h2 className="text-xs font-semibold uppercase tracking-wider text-amber-100/50">
        Status
      </h2>

      <div className="flex flex-col gap-2 text-sm">
        <div className="flex items-center gap-2">
          <Dot color={wsColor[wsStatus]} pulse={wsStatus === "connected"} />
          <span className="text-amber-50/90">Dashboard WS</span>
          <span className="ml-auto text-amber-100/50">
            {wsLabel[wsStatus]}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <Dot color={esp32Live ? "green" : "gray"} pulse={esp32Pulse} />
          <span className="text-amber-50/90">ESP32 link</span>
          <span className="ml-auto text-amber-100/50">
            {esp32Live ? "live" : "stale"}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <Dot color={phonePulse ? "amber" : "gray"} pulse={phonePulse} />
          <span className="text-amber-50/90">Phone link</span>
          <span className="ml-auto text-amber-100/50">
            {phonePulse ? "event received" : "idle"}
          </span>
        </div>
      </div>

      <div className="mt-1 grid grid-cols-2 gap-x-3 gap-y-2 border-t border-neutral-800 pt-3 text-xs">
        <span className="text-amber-100/40">Current edge</span>
        <span className="text-right font-mono text-amber-50/90">
          {edge ? edge.id : state?.arrived ? "arrived" : "—"}
        </span>

        <span className="text-amber-100/40">Progress</span>
        <span className="text-right font-mono text-amber-50/90">
          {edge ? `${edge.progress_pct.toFixed(1)}%` : "—"}
        </span>

        <span className="text-amber-100/40">Cumulative distance</span>
        <span className="text-right font-mono text-amber-50/90">
          {state ? `${state.cumulative_distance_m.toFixed(1)} m` : "—"}
        </span>

        <span className="text-amber-100/40">Stall check</span>
        <span
          className={`text-right font-mono ${
            stall?.is_stalled ? "text-red-400" : "text-amber-50/90"
          }`}
        >
          {stall
            ? stall.is_stalled
              ? `STALLED (${stall.consecutive_stall_samples})`
              : "clear"
            : "—"}
        </span>
      </div>
    </div>
  );
}
