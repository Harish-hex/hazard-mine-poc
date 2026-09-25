# hazard-mine-poc

Go backend for a 24-hour hackathon POC ("Guiding People Around Temporary Hazards"),
built around a fictional NMDC Bailadila open-cast mine setting. A cart moves along a
small hardcoded 5-node mine graph. An ESP32 (or the mock client below) streams raw
sensor telemetry over WebSocket at ~10Hz; the backend integrates that into cumulative
distance + heading, maps distance onto a predefined sequential path through the graph,
detects **stall** (motor on, wheel not turning — the core hazard signal) from the same
stream, and reacts to phone-reported hazards (HTTP POST, no hardware needed) by
blocking the cart's current edge and rerouting with Dijkstra. Live state pushes to a
dashboard over an outbound WebSocket. Everything is in-memory, no DB, no auth — a
"manual override" endpoint exists as a dead-reckoning-drift failsafe for the demo. See
`CLAUDE.md` for the full project spec.

## Running it

```bash
go run ./cmd/server                              # starts the backend on :8080
go run ./cmd/mockclient                          # fake ESP32: streams scripted telemetry to /ws/telemetry
go run ./cmd/wsdump                              # dev helper: dials /ws/dashboard and prints every frame
```

Or start the backend + dashboard together with `./dev.sh` (add `--mock` to also launch
the mock telemetry client). Ctrl-C stops everything.

Open `http://localhost:8080/phone` for the phone hazard-report page.

Run tests: `go test ./... -race`

## Endpoints

| Method & path | Description |
|---|---|
| `GET /healthz` | Liveness check. |
| `GET /api/state` | Non-WS snapshot of the current dashboard state (same shape as the WS push). |
| `GET /api/graph` | Static topology: all nodes (name/coords) and edges (weight), fetched once by the dashboard. |
| `GET /phone` | Embedded HTML hazard-report form (type + severity → POSTs to `/api/hazard`). |
| `POST /api/hazard` | Reports a hazard (`{"type": "...", "severity": "..."}`, no `edge_id`). Always blocks the cart's *current* edge and reroutes. |
| `DELETE /api/hazard/{edge_id}` | Clears a previously blocked edge and reroutes. |
| `POST /api/override` | Dashboard failsafe: force-sets the cart's current node (`{"node_id": "..."}`), resets distance-into-edge, reroutes. |
| `GET /ws/telemetry` | Inbound WebSocket: ESP32 (or mock client) streams raw telemetry samples here at ~10Hz. Single active connection — a new upgrade evicts the prior one. |
| `GET /ws/dashboard` | Outbound WebSocket: full `DashboardState` snapshot pushed on every state change. Multiple subscribers supported. |

## Trigger the demo reroute

With the server (and ideally `mockclient` + `wsdump`) running, block whatever edge the
cart is currently on:

```bash
curl -X POST localhost:8080/api/hazard \
  -H 'Content-Type: application/json' \
  -d '{"type":"rockfall","severity":"high"}'
```

Response: `{"blocked_edge": "...", "route_changed": true}` — and the dashboard WS
stream reflects the new route within one push cycle. Clear it again with:

```bash
curl -X DELETE localhost:8080/api/hazard/<edge_id>
```

## Dashboard

A Next.js dashboard lives in `dashboard/` — node-graph minimap, live vehicle marker,
blocked/route edge highlighting, status panel, event log, and a manual-override panel.
Run it alongside the backend and mock client:

```bash
cd dashboard
npm install   # first time only
npm run dev   # http://localhost:3000
```

It points at the backend via two env vars, both already defaulting to `localhost:8080`
so no `.env` is needed for local dev:

- `NEXT_PUBLIC_BACKEND_HTTP_URL` — REST base (`/api/graph`, `/api/state`, `/api/hazard`, `/api/override`)
- `NEXT_PUBLIC_BACKEND_WS_URL` — dashboard WS (`ws://localhost:8080/ws/dashboard`)

The "Manual override (failsafe)" panel is deliberately mapped onto the three real
endpoints above (force node position, force-block the current edge, clear a blocked
edge) rather than any generic edge-blocking control, since the backend doesn't expose one.

A Playwright regression spec covers the live sequence end-to-end (connect, stall
detect/clear, hazard reroute, manual override, WS reconnect). With the backend,
`mockclient`, and the dashboard dev server all running, execute it from `dashboard/`:

```bash
npx playwright test
```
