import type {
  DashboardState,
  GraphResponse,
  HazardBlockedResponse,
  HazardUnblockedResponse,
} from "./types";

const HTTP_URL =
  process.env.NEXT_PUBLIC_BACKEND_HTTP_URL ?? "http://localhost:8080";

async function asJson<T>(res: Response, label: string): Promise<T> {
  if (!res.ok) {
    let detail = "";
    try {
      detail = await res.text();
    } catch {
      // ignore
    }
    throw new Error(`${label} failed: ${res.status} ${detail}`.trim());
  }
  return res.json() as Promise<T>;
}

export async function fetchGraph(): Promise<GraphResponse> {
  const res = await fetch(`${HTTP_URL}/api/graph`);
  return asJson<GraphResponse>(res, "GET /api/graph");
}

export async function fetchState(): Promise<DashboardState> {
  const res = await fetch(`${HTTP_URL}/api/state`);
  return asJson<DashboardState>(res, "GET /api/state");
}

// postHazard blocks the cart's CURRENT edge — there is no edge_id field,
// this matches the real phone client's behavior exactly.
export async function postHazard(
  type: string,
  severity?: string
): Promise<HazardBlockedResponse> {
  const res = await fetch(`${HTTP_URL}/api/hazard`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ type, severity }),
  });
  return asJson<HazardBlockedResponse>(res, "POST /api/hazard");
}

export async function deleteHazard(
  edgeId: string
): Promise<HazardUnblockedResponse> {
  const res = await fetch(
    `${HTTP_URL}/api/hazard/${encodeURIComponent(edgeId)}`,
    { method: "DELETE" }
  );
  return asJson<HazardUnblockedResponse>(res, `DELETE /api/hazard/${edgeId}`);
}

export async function postOverride(nodeId: string): Promise<DashboardState> {
  const res = await fetch(`${HTTP_URL}/api/override`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ node_id: nodeId }),
  });
  return asJson<DashboardState>(res, "POST /api/override");
}
