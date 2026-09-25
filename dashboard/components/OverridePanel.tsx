"use client";

import { useState } from "react";
import type { DashboardState, GraphResponse } from "@/lib/types";
import { deleteHazard, postHazard, postOverride } from "@/lib/api";

interface OverridePanelProps {
  graph: GraphResponse | null;
  state: DashboardState | null;
}

// Manual-override / failsafe controls. Maps onto exactly what the backend
// supports (no arbitrary "block edge X" or "advance edge" endpoints exist),
// per the reconciliation with the real API surface. Kept visually small and
// secondary — this is the lowest-priority, hackathon-failsafe panel.
export default function OverridePanel({ graph, state }: OverridePanelProps) {
  const nodes = graph?.nodes ?? [];
  const [selectedNode, setSelectedNode] = useState<string>("");
  const [selectedBlocked, setSelectedBlocked] = useState<string>("");
  const [busy, setBusy] = useState<string | null>(null);
  const [note, setNote] = useState<string>("");

  const blockedEdges = state?.blocked_edges ?? [];

  async function handleOverride() {
    const nodeId = selectedNode || nodes[0]?.id;
    if (!nodeId) return;
    setBusy("override");
    setNote("");
    try {
      await postOverride(nodeId);
      setNote(`Overrode position → ${nodeId}`);
    } catch (err) {
      setNote(err instanceof Error ? err.message : "Override failed");
    } finally {
      setBusy(null);
    }
  }

  async function handleForceHazard() {
    setBusy("hazard");
    setNote("");
    try {
      const res = await postHazard("manual-override", "high");
      setNote(`Forced hazard → blocked ${res.blocked_edge}`);
    } catch (err) {
      setNote(err instanceof Error ? err.message : "Force hazard failed");
    } finally {
      setBusy(null);
    }
  }

  async function handleClearBlocked() {
    const edgeId = selectedBlocked || blockedEdges[0];
    if (!edgeId) return;
    setBusy("clear");
    setNote("");
    try {
      await deleteHazard(edgeId);
      setNote(`Cleared ${edgeId}`);
    } catch (err) {
      setNote(err instanceof Error ? err.message : "Clear failed");
    } finally {
      setBusy(null);
    }
  }

  return (
    <details className="rounded-lg border border-neutral-800/70 bg-[#14110d] p-3 text-xs text-amber-100/60">
      <summary className="cursor-pointer select-none text-[11px] font-semibold uppercase tracking-wider text-amber-100/40">
        Manual override (failsafe)
      </summary>

      <div className="mt-3 flex flex-col gap-2.5">
        <div className="flex items-center gap-2">
          <select
            value={selectedNode}
            onChange={(e) => setSelectedNode(e.target.value)}
            className="min-w-0 flex-1 rounded border border-neutral-700 bg-neutral-900 px-1.5 py-1 text-amber-50/90"
          >
            <option value="">current node…</option>
            {nodes.map((n) => (
              <option key={n.id} value={n.id}>
                {n.id} — {n.name}
              </option>
            ))}
          </select>
          <button
            onClick={handleOverride}
            disabled={busy !== null || nodes.length === 0}
            className="shrink-0 rounded bg-neutral-800 px-2 py-1 font-medium text-amber-50/90 hover:bg-neutral-700 disabled:opacity-40"
          >
            Apply
          </button>
        </div>

        <button
          onClick={handleForceHazard}
          disabled={busy !== null}
          className="rounded border border-red-900/60 bg-red-950/40 px-2 py-1 text-left font-medium text-red-200/90 hover:bg-red-950/70 disabled:opacity-40"
        >
          Force hazard (current edge) — manual test trigger
        </button>

        <div className="flex items-center gap-2">
          <select
            value={selectedBlocked}
            onChange={(e) => setSelectedBlocked(e.target.value)}
            disabled={blockedEdges.length === 0}
            className="min-w-0 flex-1 rounded border border-neutral-700 bg-neutral-900 px-1.5 py-1 text-amber-50/90 disabled:opacity-40"
          >
            <option value="">
              {blockedEdges.length === 0 ? "no blocked edges" : "blocked edge…"}
            </option>
            {blockedEdges.map((id) => (
              <option key={id} value={id}>
                {id}
              </option>
            ))}
          </select>
          <button
            onClick={handleClearBlocked}
            disabled={busy !== null || blockedEdges.length === 0}
            className="shrink-0 rounded bg-neutral-800 px-2 py-1 font-medium text-amber-50/90 hover:bg-neutral-700 disabled:opacity-40"
          >
            Clear
          </button>
        </div>

        {note && <p className="text-amber-100/40">{note}</p>}
      </div>
    </details>
  );
}
