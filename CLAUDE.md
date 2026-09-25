# Hazard Mine POC — Project Context

## What this is
24-hour hackathon POC for the PS "Guiding People Around Temporary Hazards" — a system
that detects a changing physical hazard on a route and reroutes around it. This is a
pivot of a previously-dropped SIH project: a fog/low-visibility mine vehicle safety
system, reframed for this PS, set at NMDC Bailadila open-cast mine (fictional/reference
setting for the pitch — not an actual deployment).

## Team
- Harish — all software solo: firmware event logic, Go backend, dashboard
- Varman — electronics/sensor wiring support
- Third teammate — 5W1H research spreadsheet (DGMS/NMDC/MSHA-type cited sources) as
  pitch/evidence document. Not part of the software build.

## Hardware (context only — not what we're building right now)
- MPU6050 (gyro/accel)
- Single wheel encoder (ASU) — only one available, so single-wheel odometry, no
  differential two-wheel setup
- Ultrasonic sensors
- ESP32-S3 (vehicle-side microcontroller)
- Raspberry Pi 5 (hub — runs the Go backend, will host a WiFi AP)
- Explicitly NOT available: GPS, LoRa, radar
- Testing on flat ground — separate from the simulated route graph (not a real mine)

## Core hazard signal: STALL
Hazard = vehicle STALL: motor is commanded on, but the wheel encoder shows ~no movement.
Chosen over static-obstacle detection because it's a genuinely temporary, path-blocking
event that fits the PS. Demo trigger: physically hold/lift the wheel so the encoder
gives no output while the motor is powered.

Known, acknowledged, NOT solved in this prototype: wheel-slip (wheel spins, encoder
shows movement, but chassis isn't actually displacing). This is a named limitation in
the pitch, not something to engineer around right now.

## Two hazard-reporting nodes
1. **Vehicle (ESP32-S3)** — continuous telemetry stream; backend derives stall from it.
2. **Phone** — connects to the Pi 5's WiFi, simple web form/button, POSTs a hazard event
   directly to the backend. (Chose this over BLE and over real phone GPS — GPS coords
   wouldn't map meaningfully onto the simulated graph.)

## Two hazard tiers (distinct — do not conflate)
1. **STALL (encoder-derived) — MAJOR, blocking.** `motor_pwm > threshold AND encoder_delta
   ≈ 0`, debounced over N consecutive packets. Blocks the whole current edge, triggers
   Dijkstra reroute, shown as red/dashed edge on the dashboard. This is the rerouting
   mechanism.
2. **OBSTACLE (ultrasonic-derived) — MINOR, advisory only.** `ultrasonic_cm < threshold`
   WHILE `encoder_delta` shows normal movement (i.e. vehicle is NOT stalled — this is
   what distinguishes it from tier 1). Represents small hazards: potholes, debris, etc.
   Does NOT block the edge and does NOT trigger a reroute — the edge stays fully
   traversable. Instead, logged as a **point marker** (specific spot along an edge, not
   the whole edge) onto a shared hazard map, visible on the dashboard as a small icon
   along the road curve, informational for anyone using that route later.
   - **Debounce**: cooldown by distance — don't log a new obstacle point on the same
     edge within ~20cm (tune as needed) of the last logged point, to avoid one physical
     pothole producing a cluster of duplicate markers as ultrasonic dips below threshold
     across several consecutive packets while passing over it.
   - Point data shape: `{ edge, progress (0-1 along that edge), type: "obstacle", ts }`
   - No expiry/decay logic in this POC — points accumulate for the whole demo run. Stated
     simplification, not a bug.
   - Rendering reuses the exact same `getPointAtLength()` mechanic as the moving vehicle
     dot (see Dashboard section below) — just applied to static points instead of one
     moving one.

Phone-reported hazard events also carry a type field (already in the design) — use the
same MAJOR/MINOR distinction there too, so a person-reported hazard can be either
blocking or advisory.

## Route graph
Small **hardcoded 5-node graph** (not a real mine layout) — nodes + weighted edges,
defined in code/config, not loaded from any external data source.

## Position mapping (real → simulated)
The vehicle's real-world dead-reckoned position does NOT correspond to real graph
coordinates (test ground ≠ simulated route). Mapping approach:
- **Predefined sequential path** through the graph (e.g. node0 → node1 → node2 → ...)
- Track **cumulative distance traveled** (from encoder integration)
- Backend flips `current_edge` when cumulative distance crosses that edge's length
  threshold
- A **manual override control** on the dashboard exists as a hackathon failsafe only
  (in case dead-reckoning drifts during the actual demo run) — not the primary
  mechanism.

## Direction is commanded, not sensed (stated assumption — read before building)
Available sensors (single wheel encoder + gyro yaw rate) CANNOT detect which fork a
vehicle physically took at a junction — that needs GPS, distinguishable turn geometry,
or vision, none of which exist here.
- Which edge is "current" is never inferred from sensor data. It is simply the next
  unconsumed edge in whatever route Dijkstra currently has active — software state, not
  physical detection.
- Cumulative encoder distance only answers "how far along the current (software-picked)
  edge," never "which edge." Once distance crosses that edge's length, the backend
  advances to the next edge in the plan — regardless of what the vehicle physically did.
- Consequence: the RC car cannot "choose" a fork in this design — there is no wrong way,
  because no real branch-following happens. It can physically drive straight (e.g. on a
  short track/roller) for the entire demo; only encoder ticks matter, not real
  trajectory or which junction it's "at."
- Stated explicitly in the pitch as a scoped limitation: real branch-following would
  need waypoint navigation or GPS at junctions — out of scope here. This POC
  demonstrates the sensing + rerouting logic assuming the commanded route is followed,
  not real-time physical path selection.



```
ESP32-S3 --(WiFi, WebSocket, continuous telemetry, ~10Hz)--> Go backend (Pi 5)
Phone     --(WiFi, HTTP POST, event-only)--------------------> Go backend (Pi 5)
Go backend --(WebSocket, live state push)---------------------> Next.js dashboard
```

- Pi 5 runs the WiFi AP; both ESP32 and phone join it as clients.
- WS (not polling HTTP) for vehicle telemetry — need a continuous stream for
  dead-reckoning integration.
- HTTP POST is fine for phone events — one-off, not a stream.
- **In-memory state only** on the backend. No DB. Restart = reset. This is fine for a
  24hr demo.

## Telemetry packet (ESP32 → backend, via WS, ~10Hz)
```json
{
  "encoder_delta": 0,
  "gyro_z": 0.0,
  "motor_pwm": 0,
  "ultrasonic_cm": 0.0,
  "ts": 0
}
```
- `encoder_delta`: raw tick delta since last packet
- `gyro_z`: yaw rate from MPU6050
- `motor_pwm`: current commanded motor power (used to detect "motor on but not moving")
- `ultrasonic_cm`: distance reading — drives MINOR obstacle detection (separate from
  stall logic; see "Two hazard tiers" above)
- `ts`: device timestamp (ms)

All odometry integration happens **on the backend, not on the ESP32**. Firmware just
ships raw readings.

## Backend responsibilities (Go, on Pi 5)
1. **WS server (inbound)** — ingest ESP32 telemetry stream
2. **HTTP endpoint (inbound)** — ingest phone hazard POSTs
3. **Odometry module** — integrate `encoder_delta` → distance (via wheel
   circumference/ticks-per-rev), integrate `gyro_z` → heading; maintain cumulative
   distance
4. **Edge-tracking** — map cumulative distance to current graph edge via the predefined
   sequential path + per-edge length thresholds
5. **Stall detector (MAJOR hazard)** — debounced check: `motor_pwm > threshold AND
   encoder_delta ≈ 0` for N consecutive packets (avoid single noisy-sample false
   triggers)
6. **Obstacle detector (MINOR hazard)** — `ultrasonic_cm < threshold` while
   `encoder_delta` shows normal movement; distance-based cooldown (~20cm) to avoid
   duplicate points for the same physical obstacle; appends `{edge, progress, type, ts}`
   to an in-memory hazard-points list — does NOT touch the graph/blocked-edges state
7. **Graph module** — 5-node hardcoded graph, weighted edges, block/unblock an edge on a
   confirmed MAJOR (stall) event only
8. **Reroute** — **Dijkstra** (weighted shortest path) recomputed whenever an edge gets
   blocked
9. **WS server (outbound)** — push live state to the dashboard: vehicle position,
   current edge, blocked edges, current best route, and the accumulated hazard-points
   list

## Dashboard (Next.js)

### Visual concept: game-style minimap, not an abstract node graph
The route is rendered as an actual winding road (hand-drawn SVG bezier curves per edge),
with the vehicle as a dot that glides along the curve — like a minimap path-follow in a
game, not straight lines between circles. A working prototype validating this exact
technique exists here: https://claude.ai/artifact/RUqegLHonqwEziFpjaRbqv — port its
pattern directly into a React component rather than rebuilding from scratch.

**Core mechanic**: each edge is an SVG `<path>` (bezier curve). On mount, call
`path.getTotalLength()` once per edge and cache it. To place the vehicle, call
`path.getPointAtLength(progress * totalLength)` — this returns `{x, y}` and the dot
follows the curve's actual shape (corners included) with zero physics/animation-library
code. Update the `<circle cx cy>` on every WS message from the backend.

**Critical decoupling — read this before building**: the dot's position depends on
ONE scalar only: `progress` (0→1) along the current edge, derived purely from
`cumulative_encoder_distance / edge_weight`. The SVG curve shape is 100% cosmetic and
carries no positional/directional truth — it's the same trick transit maps use (real
topology, decorative line shape). This means:
- The real RC car can drive in a straight line on flat test ground the entire time,
  while the dashboard shows it winding around a mine hairpin bend — no conflict,
  because heading/gyro data is never used to place the dot.
- `gyro_z` is NOT required to drive the dot's position. Optional uses only: (a) a
  sanity/integrity check — heading should stay flat during a straight-line test run,
  drift signals sensor noise; (b) purely cosmetic vehicle-icon rotation to face the
  curve's tangent direction (nice-to-have, not required for the core demo).
- Go-side implication: define edge weights in the same units as the encoder-derived
  distance (e.g. cm) so `progress = cumulative_distance / edge_weight` is a clean 0→1
  ratio. Don't mix abstract "graph units" with real encoder units.

### Layout (single screen, no scrolling — built for a live pitch demo)
```
┌─────────────────────────────────────────────┬──────────────┐
│                                               │  STATUS      │
│                                               │  panel       │
│         MINIMAP (main, curved road, ~70%)    ├──────────────┤
│                                               │  EVENT LOG   │
│                                               │  (scrolls)   │
├───────────────────────────────────────────────┤              │
│  ROUTE STRIP (current path breadcrumb, thin)  │              │
└─────────────────────────────────────────────┴──────────────┘
```

**1. Minimap (centerpiece)**
- 5 nodes + edges as curved SVG paths, fixed hand-placed coordinates (no live
  force-layout — jitter reads as broken on stage)
- Vehicle marker via `getPointAtLength()`, per the mechanic above
- **Visual treatment — "lit tunnel," not a flat all-edges-visible graph.** Rendering
  every possible edge at equal visual weight (as in the first mock) implies the vehicle
  chooses a fork in real time, which it never does (see "Direction is commanded, not
  sensed" above) — the visual should represent that honestly, mine-headlamp style:
  - **Active route** (Dijkstra's current pick): bright, thick, glowing stroke (the blue
    treatment already used)
  - **All other edges**: dim, thin, low-opacity outline — always rendered (so structure
    is visible and future reroutes have somewhere to reveal), but visually recessive.
    Reads as "known tunnels," not "choices available right now"
  - **Blocked edge**: red/dashed, and its glow (if it was previously lit/active) should
    extinguish as part of the same transition
  - **On reroute**: animate a brief lighting sweep along the newly-active path (progressive
    opacity/glow fade-in from the vehicle's position outward), timed with the reroute
    log entry — this is the visual "aha" moment for judges, replaces having to read a log
    line to notice a reroute happened
  - **Vehicle dot**: small radial glow/headlamp-cone effect — reinforces the metaphor,
    adds presence with no extra assets needed
  - **Junction nodes**: style as mine support/junction icons rather than plain circles
  - Side benefit: dimming inactive edges also fixes visual clutter at busy junctions
    where several edges converge and cross (a real problem in a flat equal-weight graph)
- Node labels always visible; edge weights on hover/tap only
- **Minor hazard points** (from the ultrasonic obstacle tier): small static warning
  icons plotted along an edge's curve via the same `getPointAtLength()` call used for
  the vehicle dot, just evaluated once per point instead of every frame. These do NOT
  affect edge color/state — the edge stays "active" blue even with obstacle markers on
  it, since obstacles are advisory, not blocking.

**2. Status panel (top-right)**
- Connection indicators: ESP32 link, phone link — dot + label, blink on packet receipt
  (this is the "prove it's actually live" signal for judges)
- Live readouts: current edge, cumulative distance, progress %, last stall-check result

**3. Event log (below status panel)**
- Timestamped feed: stall detected, route recalculated, hazard cleared
- Newest on top, auto-scroll — this is what you narrate from during the pitch

**4. Route strip (bottom)**
- Current best path as breadcrumb, e.g. `A → B → D → E`
- Flash/highlight on change so a reroute is visually obvious without explanation

**5. Manual override (small, unobtrusive)**
- Buttons to manually advance edge / force-block an edge
- Failsafe only — keep it visually secondary to the "real" system

### Necessary features, ranked
1. WS client with auto-reconnect (a dropped connection mid-pitch is the worst failure
   mode)
2. Live vehicle position + edge-state rendering (the minimap mechanic above)
3. Blocked-edge + reroute visual feedback — has to read instantly from across a room
4. Event log (narrates state changes judges won't infer from the map alone)
5. Connection/status indicators (proves it's live, not canned)
6. Manual override (safety net, lowest priority — build/stub last)

### Explicitly skip for this POC
Auth, responsive/mobile layout, historical playback, settings/config UI — none of it
earns its build time in 24 hours.

## Phone node
- Static HTML page served by the Go backend (or a route on the same server)
- One button/form → POST a hazard event (include a hazard type field)

## Build order (agreed)
1. **Go backend first, standalone** — graph + Dijkstra + WS/HTTP server — tested with a
   **mocked telemetry generator** (a script spamming fake WS packets), no Pi/hardware
   needed yet. This is the piece everything else plugs into, and reroute logic can be
   verified before any hardware is wired up.
2. Firmware (ESP32-S3) — once backend ingest is solid.
3. Dashboard — wire to the real WS feed last.
4. Pi 5 setup (WiFi AP via hostapd/dnsmasq, deploy backend) happens when moving from
   mocked telemetry to real hardware — not needed for early backend dev.

## Pitch framing (context, not implementation)
Presented as a "limited prototype proof of concept" — explicitly stated that with
proper budget (RTK GPS, LoRa, mmWave radar, dual encoders) the system would be far more
accurate/precise. Reference site: NMDC Bailadila open-cast mine.

## Explicitly out of scope for this build
- Sourcing/assembling actual hardware (separate track)
- Wheel-slip detection/correction
- Real mine layout data / real GPS
- Any persistence layer beyond in-memory state
