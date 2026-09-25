// Static minimap geometry: projects the graph's raw (x,y) node coordinates
// into a fixed SVG viewBox, and authors one cubic-bezier "d" string per edge
// with a fixed perpendicular offset for a hand-drawn "winding road" look.
// Both are computed ONCE from the fetched graph topology — never
// recomputed per frame, never randomized (jitter reads as broken on stage).

export interface ProjectedNode {
  id: string;
  name: string;
  x: number;
  y: number;
}

export interface EdgeGeometry {
  id: string;
  a: string;
  b: string;
  weightM: number;
  d: string;
}

export const VIEWBOX_W = 1000;
export const VIEWBOX_H = 600;
export const MINIMAP_VIEWBOX = `0 0 ${VIEWBOX_W} ${VIEWBOX_H}`;

const MARGIN = 110;
const CURVE_OFFSET_RATIO = 0.15;

export function projectNodes(
  nodes: { id: string; name: string; x: number; y: number }[]
): ProjectedNode[] {
  if (nodes.length === 0) return [];

  const xs = nodes.map((n) => n.x);
  const ys = nodes.map((n) => n.y);
  const minX = Math.min(...xs);
  const maxX = Math.max(...xs);
  const minY = Math.min(...ys);
  const maxY = Math.max(...ys);

  const spanX = maxX - minX || 1;
  const spanY = maxY - minY || 1;

  const usableW = VIEWBOX_W - 2 * MARGIN;
  const usableH = VIEWBOX_H - 2 * MARGIN;

  return nodes.map((n) => ({
    id: n.id,
    name: n.name,
    x: MARGIN + ((n.x - minX) / spanX) * usableW,
    y: MARGIN + ((n.y - minY) / spanY) * usableH,
  }));
}

export function buildEdgeGeometry(
  edges: { id: string; a: string; b: string; weight_m: number }[],
  projected: ProjectedNode[]
): EdgeGeometry[] {
  const byId = new Map(projected.map((n) => [n.id, n]));

  return edges.map((e, i) => {
    const a = byId.get(e.a);
    const b = byId.get(e.b);
    if (!a || !b) {
      return { id: e.id, a: e.a, b: e.b, weightM: e.weight_m, d: "" };
    }

    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const len = Math.sqrt(dx * dx + dy * dy) || 1;

    // Unit vector perpendicular to A->B.
    const px = -dy / len;
    const py = dx / len;

    // Fixed, deterministic alternation per edge index — a hand-computed
    // constant, never randomized or recomputed per frame.
    const sign = i % 2 === 0 ? 1 : -1;
    const offset = len * CURVE_OFFSET_RATIO * sign;

    const c1x = a.x + dx * 0.33 + px * offset;
    const c1y = a.y + dy * 0.33 + py * offset;
    const c2x = a.x + dx * 0.66 + px * offset;
    const c2y = a.y + dy * 0.66 + py * offset;

    const d = `M ${a.x} ${a.y} C ${c1x} ${c1y}, ${c2x} ${c2y}, ${b.x} ${b.y}`;

    return { id: e.id, a: e.a, b: e.b, weightM: e.weight_m, d };
  });
}
