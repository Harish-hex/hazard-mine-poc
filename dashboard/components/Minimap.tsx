"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { DashboardState, GraphResponse } from "@/lib/types";
import {
  MINIMAP_VIEWBOX,
  buildEdgeGeometry,
  projectNodes,
  type EdgeGeometry,
} from "@/lib/geometry";

interface MinimapProps {
  graph: GraphResponse | null;
  state: DashboardState | null;
}

interface VehiclePos {
  x: number;
  y: number;
}

// Six-point "mine junction" support-frame icon (pointy-top hexagon), drawn
// once per node from its fixed projected coordinates — same "compute once,
// never per-frame" rule as the edge geometry in lib/geometry.ts.
function hexPoints(cx: number, cy: number, r: number): string {
  const pts: string[] = [];
  for (let i = 0; i < 6; i++) {
    const angle = (Math.PI / 3) * i - Math.PI / 2;
    pts.push(`${cx + r * Math.cos(angle)},${cy + r * Math.sin(angle)}`);
  }
  return pts.join(" ");
}

export default function Minimap({ graph, state }: MinimapProps) {
  const projectedNodes = useMemo(
    () => (graph ? projectNodes(graph.nodes) : []),
    [graph]
  );

  const edgeGeoms = useMemo(
    () => (graph ? buildEdgeGeometry(graph.edges, projectedNodes) : []),
    [graph, projectedNodes]
  );

  const pathRefs = useRef<Map<string, SVGPathElement>>(new Map());
  const lengthsRef = useRef<Map<string, number>>(new Map());
  const [pathsReady, setPathsReady] = useState(false);

  // Cache each edge path's total length ONCE, right after the (fixed)
  // geometry mounts. This is the entire setup step for the positioning
  // mechanic — no re-measurement afterward.
  useEffect(() => {
    if (edgeGeoms.length === 0) return;
    lengthsRef.current.clear();
    for (const eg of edgeGeoms) {
      const el = pathRefs.current.get(eg.id);
      if (el) {
        lengthsRef.current.set(eg.id, el.getTotalLength());
      }
    }
    // This reads an imperative DOM API (getTotalLength) that only exists
    // after the SVG paths mount; there's no pure-render equivalent.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPathsReady(true);
  }, [edgeGeoms]);

  const [vehiclePos, setVehiclePos] = useState<VehiclePos | null>(null);
  const [vehicleRotationDeg, setVehicleRotationDeg] = useState(0);

  // The ENTIRE vehicle-positioning mechanic: progress (0-1) along the
  // current edge's authored path -> getPointAtLength(). No physics, no
  // tweening beyond the CSS transition applied to cx/cy below.
  useEffect(() => {
    if (!state || !pathsReady) return;

    if (state.arrived || !state.current_edge) {
      const exit = projectedNodes.find((n) => n.id === "EXIT");
      // Derived from an imperative DOM read (path geometry), same as below.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      if (exit) setVehiclePos({ x: exit.x, y: exit.y });
      return;
    }

    const edge = state.current_edge;
    const geom = edgeGeoms.find((eg) => eg.id === edge.id);
    const path = geom ? pathRefs.current.get(geom.id) : undefined;
    const totalLength = geom ? lengthsRef.current.get(geom.id) : undefined;

    if (!geom || !path || totalLength === undefined) return;

    const progress = Math.min(1, Math.max(0, edge.progress_pct / 100));
    // Path is authored A->B. If the cart's last-passed node is B, it's
    // traveling B->A, so progress 0 corresponds to the END of the path.
    const forward = state.current_node === geom.a;
    const t = forward ? progress : 1 - progress;

    const s = t * totalLength;
    const point = path.getPointAtLength(s);
    setVehiclePos({ x: point.x, y: point.y });

    // Cosmetic-only rotation, per CLAUDE.md: face the CURVE'S OWN tangent
    // direction at the vehicle's position, never the raw gyro-derived
    // heading_deg. The whole minimap mechanic depends on real-world heading
    // and the simulated curve's direction being decoupled (flat-ground test
    // rig, winding simulated route) — rotating by heading_deg would make the
    // icon visibly fight the curve on every turn, which is exactly the
    // coupling this design is meant to avoid. Sample two nearby points along
    // the path in the direction of travel and use their tangent instead.
    const eps = 2;
    const sBack = Math.max(0, s - eps);
    const sFwd = Math.min(totalLength, s + eps);
    const pBack = path.getPointAtLength(sBack);
    const pFwd = path.getPointAtLength(sFwd);
    const sign = forward ? 1 : -1;
    const dx = (pFwd.x - pBack.x) * sign;
    const dy = (pFwd.y - pBack.y) * sign;
    if (dx !== 0 || dy !== 0) {
      const tangentDeg = (Math.atan2(dy, dx) * 180) / Math.PI;
      // Icon's local "forward" (the headlamp cone) points in -y (-90deg);
      // rotate() is clockwise in SVG's y-down space, matching atan2 directly.
      setVehicleRotationDeg(tangentDeg + 90);
    }
  }, [state, pathsReady, edgeGeoms, projectedNodes]);

  const routeEdgeSet = useMemo(
    () => new Set(state?.route.edges ?? []),
    [state]
  );
  const blockedEdgeSet = useMemo(
    () => new Set(state?.blocked_edges ?? []),
    [state]
  );

  // Lighting-sweep trigger: bump sweepSeq whenever the route's edge list
  // actually changes (reroute), but not on the very first snapshot — same
  // "baseline, don't spam on connect" rule as useEventLog/RouteStrip's flash.
  const [sweepSeq, setSweepSeq] = useState(0);
  const prevRouteKeyRef = useRef<string | null>(null);
  useEffect(() => {
    if (!state) return;
    const key = state.route.edges.join(",");
    if (prevRouteKeyRef.current !== null && prevRouteKeyRef.current !== key) {
      setSweepSeq((s) => s + 1);
    }
    prevRouteKeyRef.current = key;
  }, [state]);

  function edgeClassName(eg: EdgeGeometry): string {
    if (blockedEdgeSet.has(eg.id)) {
      return "stroke-red-500 [stroke-dasharray:10_8]";
    }
    if (routeEdgeSet.has(eg.id)) {
      return "stroke-sky-300";
    }
    // Inactive tunnel wall: dimmed almost to the background so the lit
    // route reads as the one path with light in it.
    return "stroke-neutral-800/70";
  }

  function edgeStrokeWidth(eg: EdgeGeometry): number {
    if (blockedEdgeSet.has(eg.id)) return 6;
    if (routeEdgeSet.has(eg.id)) return 7;
    return 3;
  }

  // Glow filter: applied only to the active route (lit tunnel light) and
  // blocked/hazard edges (danger glow) — inactive edges stay flat/dim.
  function edgeFilter(eg: EdgeGeometry): string | undefined {
    if (blockedEdgeSet.has(eg.id)) return "url(#glow-hazard)";
    if (routeEdgeSet.has(eg.id)) return "url(#glow-route)";
    return undefined;
  }

  const stalled = state?.stall.is_stalled ?? false;

  return (
    <div className="relative h-full w-full overflow-hidden rounded-lg border border-neutral-800 bg-[#171310]">
      {/* Title overlay */}
      <div className="pointer-events-none absolute left-4 top-4 z-10">
        <h1 className="text-sm font-semibold tracking-wide text-amber-100/90">
          Hazard Mine POC
        </h1>
        <p className="text-xs text-amber-100/50">
          NMDC Bailadila (simulated)
        </p>
      </div>

      <svg
        viewBox={MINIMAP_VIEWBOX}
        className="h-full w-full"
        preserveAspectRatio="xMidYMid meet"
      >
        <defs>
          <radialGradient id="dirt-bg" cx="50%" cy="40%" r="75%">
            <stop offset="0%" stopColor="#241d17" />
            <stop offset="100%" stopColor="#100d0a" />
          </radialGradient>

          {/* Route "tunnel light" glow */}
          <filter id="glow-route" x="-60%" y="-60%" width="220%" height="220%">
            <feGaussianBlur stdDeviation="5" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>

          {/* Blocked/hazard danger glow */}
          <filter id="glow-hazard" x="-60%" y="-60%" width="220%" height="220%">
            <feGaussianBlur stdDeviation="4.5" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>

          {/* Vehicle headlamp / sweep-pulse glow */}
          <filter id="headlamp-glow" x="-200%" y="-200%" width="500%" height="500%">
            <feGaussianBlur stdDeviation="8" />
          </filter>
        </defs>
        <rect width={1000} height={600} fill="url(#dirt-bg)" />

        {/* Edges */}
        {edgeGeoms.map((eg) => (
          <path
            key={eg.id}
            ref={(el) => {
              if (el) pathRefs.current.set(eg.id, el);
              else pathRefs.current.delete(eg.id);
            }}
            d={eg.d}
            fill="none"
            strokeWidth={edgeStrokeWidth(eg)}
            strokeLinecap="round"
            filter={edgeFilter(eg)}
            className={`transition-colors duration-300 ${edgeClassName(eg)}`}
          >
            <title>
              {eg.a} → {eg.b}: {eg.weightM} m
            </title>
          </path>
        ))}

        {/* Reroute lighting sweep: a bright pulse traveling once along each
            newly (re)lit route edge. Keyed on sweepSeq so React remounts
            (and SMIL restarts) the animation every time the route changes;
            it does not fire on the initial route computation. */}
        {sweepSeq > 0 &&
          edgeGeoms
            .filter((eg) => routeEdgeSet.has(eg.id))
            .map((eg) => (
              <circle
                key={`${eg.id}-sweep-${sweepSeq}`}
                r={9}
                filter="url(#headlamp-glow)"
                className="fill-amber-100"
              >
                <animateMotion
                  dur="0.9s"
                  path={eg.d}
                  fill="freeze"
                  calcMode="linear"
                />
                <animate
                  attributeName="opacity"
                  values="0;1;0.9;0"
                  keyTimes="0;0.15;0.8;1"
                  dur="0.9s"
                  fill="freeze"
                />
              </circle>
            ))}

        {/* Nodes — mine-junction support-frame icons */}
        {projectedNodes.map((n) => (
          <g key={n.id}>
            <polygon
              points={hexPoints(n.x, n.y, 15)}
              className="fill-[#20180f] stroke-amber-200/70"
              strokeWidth={2.5}
            />
            <line
              x1={n.x - 9}
              y1={n.y - 9}
              x2={n.x + 9}
              y2={n.y + 9}
              className="stroke-amber-200/30"
              strokeWidth={1.5}
            />
            <line
              x1={n.x - 9}
              y1={n.y + 9}
              x2={n.x + 9}
              y2={n.y - 9}
              className="stroke-amber-200/30"
              strokeWidth={1.5}
            />
            <circle cx={n.x} cy={n.y} r={4.5} className="fill-amber-100/90" />
            <text
              x={n.x}
              y={n.y - 24}
              textAnchor="middle"
              className="fill-amber-50/90 text-[15px] font-medium"
              style={{ paintOrder: "stroke", stroke: "#100d0a", strokeWidth: 4 }}
            >
              {n.name}
            </text>
          </g>
        ))}

        {/* Vehicle marker + headlamp glow. The cone is rotated to face the
            curve's own tangent direction at the vehicle's position (computed
            in the effect above) — cosmetic only, and deliberately NOT tied to
            the raw gyro heading_deg (see the comment on that computation). */}
        {vehiclePos && (
          <g
            transform={`translate(${vehiclePos.x} ${vehiclePos.y}) rotate(${vehicleRotationDeg})`}
            style={{ transition: "transform 0.15s linear" }}
          >
            <path
              d="M 0 0 L -30 -52 A 34 34 0 0 1 30 -52 Z"
              filter="url(#headlamp-glow)"
              className={stalled ? "fill-red-500/30" : "fill-amber-100/35"}
              style={{ transition: "fill 0.3s ease" }}
            />
            <circle
              r={11}
              className={
                stalled
                  ? "fill-red-500 stroke-red-100"
                  : "fill-lime-400 stroke-lime-100"
              }
              strokeWidth={3}
              style={{ transition: "fill 0.3s ease" }}
            />
          </g>
        )}
      </svg>
    </div>
  );
}
