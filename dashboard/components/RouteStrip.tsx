"use client";

import { useEffect, useRef, useState } from "react";
import type { DashboardState, GraphResponse } from "@/lib/types";

interface RouteStripProps {
  graph: GraphResponse | null;
  state: DashboardState | null;
}

export default function RouteStrip({ graph, state }: RouteStripProps) {
  const [flash, setFlash] = useState(false);
  const prevEdgesKey = useRef<string | null>(null);
  const flashTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!state) return;
    const key = state.route.edges.join(",");
    if (prevEdgesKey.current !== null && prevEdgesKey.current !== key) {
      setFlash(true);
      if (flashTimer.current) clearTimeout(flashTimer.current);
      flashTimer.current = setTimeout(() => setFlash(false), 1400);
    }
    prevEdgesKey.current = key;
  }, [state]);

  const nameById = new Map((graph?.nodes ?? []).map((n) => [n.id, n.name]));
  const nodes = state?.route.nodes ?? [];
  const label =
    nodes.length > 0
      ? nodes.map((id) => nameById.get(id) ?? id).join("  →  ")
      : "—";

  return (
    <div
      className={`flex h-full items-center gap-3 rounded-lg border px-4 text-sm transition-colors duration-300 ${
        flash
          ? "border-sky-400 bg-sky-950/60 text-sky-100"
          : "border-neutral-800 bg-[#171310] text-amber-100/80"
      }`}
    >
      <span className="shrink-0 text-xs uppercase tracking-wider text-amber-100/40">
        Route
      </span>
      <span className="truncate font-medium">{label}</span>
      {state?.route.total_distance_m !== undefined && (
        <span className="ml-auto shrink-0 text-xs text-amber-100/40">
          {state.route.total_distance_m.toFixed(1)} m total
        </span>
      )}
    </div>
  );
}
