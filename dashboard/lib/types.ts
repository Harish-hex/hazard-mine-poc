// Wire types mirroring the Go backend's JSON shapes exactly.
// Source of truth: internal/httpapi/static.go (graphResponse) and
// internal/state/dashboard_state.go (DashboardState).

export type NodeId = string;
export type EdgeId = string;

export interface GraphNode {
  id: NodeId;
  name: string;
  x: number;
  y: number;
}

export interface GraphEdge {
  id: EdgeId;
  a: NodeId;
  b: NodeId;
  weight_m: number;
}

export interface GraphResponse {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface CurrentEdgeInfo {
  id: EdgeId;
  edge_length_m: number;
  distance_into_edge_m: number;
  progress_pct: number;
}

export interface RouteInfo {
  nodes: NodeId[];
  edges: EdgeId[];
  total_distance_m: number;
}

export interface StallInfo {
  is_stalled: boolean;
  consecutive_stall_samples: number;
}

export interface TelemetryInfo {
  ultrasonic_cm: number;
  motor_pwm: number;
  last_update_ts: number;
}

export type HazardSource = "phone" | "vehicle" | string;

export interface HazardInfo {
  edge_id: EdgeId;
  type: string;
  source: HazardSource;
  at: string;
}

export interface DashboardState {
  timestamp: string;
  current_node: NodeId;
  heading_deg: number;
  cumulative_distance_m: number;
  current_edge: CurrentEdgeInfo | null;
  route: RouteInfo;
  blocked_edges: EdgeId[];
  stall: StallInfo;
  telemetry: TelemetryInfo;
  last_hazard: HazardInfo | null;
  arrived: boolean;
}

export interface HazardBlockedResponse {
  blocked_edge: string;
  route_changed: boolean;
}

export interface HazardUnblockedResponse {
  unblocked_edge: string;
  route_changed: boolean;
}
