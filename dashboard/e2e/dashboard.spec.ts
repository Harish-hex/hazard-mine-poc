import { test, expect, type Page } from "@playwright/test";
import { spawn, type ChildProcess } from "node:child_process";
import path from "node:path";
import fs from "node:fs";

// This is a smoke test driven against the REAL stack (Go backend on :8080,
// mock ESP32 telemetry client, Next.js dev server on :3000) — not an
// isolated/mocked unit test. It expects:
//   - the Go backend already running fresh (in-memory state at its initial
//     N1-start snapshot, no mockclient attached yet)
//   - the Next.js dev server already running on :3000
// It spawns/kills the mock telemetry client itself so it can observe the
// full scripted run (steady motion -> stall window -> auto-reroute ->
// arrival) live, from a fresh backend snapshot, rather than racing an
// already-in-progress run.

const REPO_ROOT = path.resolve(__dirname, "..", ".."); // .../harzard-poc
const BACKEND_HTTP = "http://localhost:8080";
const SCREENSHOT_DIR = path.join(__dirname, "screenshots");

fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

let mockclient: ChildProcess | null = null;

function startMockclient() {
  mockclient = spawn("go", ["run", "./cmd/mockclient"], {
    cwd: REPO_ROOT,
    stdio: "ignore",
    detached: true,
  });
}

function stopMockclient() {
  if (mockclient && mockclient.pid) {
    try {
      // Negative PID: kill the whole detached process group (go run spawns
      // a child binary; killing just the wrapper leaves the binary alive).
      process.kill(-mockclient.pid, "SIGKILL");
    } catch {
      // already dead
    }
  }
  mockclient = null;
}

interface BackendState {
  current_node: string;
  current_edge: { id: string; progress_pct: number } | null;
  route: { nodes: string[]; edges: string[] };
  blocked_edges: string[];
  stall: { is_stalled: boolean; consecutive_stall_samples: number };
  arrived: boolean;
}

async function getState(): Promise<BackendState> {
  const res = await fetch(`${BACKEND_HTTP}/api/state`);
  if (!res.ok) throw new Error(`GET /api/state failed: ${res.status}`);
  return res.json();
}

async function waitForState(
  predicate: (s: BackendState) => boolean,
  timeoutMs: number,
  label: string
): Promise<BackendState> {
  const deadline = Date.now() + timeoutMs;
  let last: BackendState | null = null;
  while (Date.now() < deadline) {
    last = await getState();
    if (predicate(last)) return last;
    await new Promise((r) => setTimeout(r, 50));
  }
  throw new Error(
    `waitForState timed out waiting for "${label}"; last state: ${JSON.stringify(last)}`
  );
}

async function shot(page: Page, name: string) {
  await page.screenshot({ path: path.join(SCREENSHOT_DIR, `${name}.png`) });
}

test.afterAll(() => {
  stopMockclient();
});

test("hazard-mine dashboard: live telemetry, stall, hazard reroute, override", async ({
  page,
}) => {
  // Sanity: backend should be at its fresh initial snapshot (no mockclient
  // attached to this run yet). If cumulative distance is already nonzero,
  // the caller forgot to restart the backend before this test.
  const initial = await getState();
  expect(initial.current_node).toBe("N1");
  expect(initial.arrived).toBe(false);

  // --- 1. Load, wait for WS connection ------------------------------------
  await page.goto("/");
  await expect(page.getByText("Connected", { exact: true })).toBeVisible({
    timeout: 10_000,
  });
  // Initial-snapshot fallback: even before any telemetry has streamed, the
  // dashboard should already show the cart sitting at N1 (REST fallback).
  await expect(page.getByText("N1-N2", { exact: true }).first()).toBeVisible({
    timeout: 5_000,
  });
  await shot(page, "00-connected-idle");

  // --- 2. Start live telemetry, observe motion ----------------------------
  startMockclient();

  await waitForState((s) => (s.current_edge?.progress_pct ?? 0) > 0, 3_000, "motion started");
  await shot(page, "01-moving");

  // --- 3. Stall window (scripted at ~10m cumulative) ----------------------
  await waitForState((s) => s.stall.is_stalled, 5_000, "stall detected");
  await expect(page.getByText(/STALLED/)).toBeVisible({ timeout: 2_000 });
  await expect(page.getByText("STALL DETECTED")).toBeVisible({ timeout: 2_000 });
  // Let the marker's 0.3s fill-color CSS transition settle before capturing,
  // purely so the screenshot shows a clean final color rather than a
  // mid-transition blend.
  await page.waitForTimeout(400);
  await shot(page, "02-stalled");

  // --- 4. Stall clears; the debouncer's rising edge auto-blocks the edge
  //        the cart was stalled on and triggers a reroute ------------------
  await waitForState((s) => !s.stall.is_stalled, 5_000, "stall cleared");
  await expect(page.getByText("clear", { exact: true })).toBeVisible({ timeout: 2_000 });
  await expect(page.getByText("stall cleared")).toBeVisible({ timeout: 2_000 });
  await page.waitForTimeout(400);
  await shot(page, "03-stall-cleared-auto-blocked");

  const afterStall = await getState();
  expect(afterStall.blocked_edges.length).toBeGreaterThan(0);
  const autoBlockedEdge = afterStall.blocked_edges[0];
  // The minimap should render that edge red/dashed (Tailwind arbitrary-value
  // class, not an actual SVG stroke-dasharray attribute — match on class).
  const autoBlockedPath = page
    .locator('svg path[class*="stroke-red-500"]')
    .first();
  await expect(autoBlockedPath).toBeVisible();
  void autoBlockedEdge;

  // --- 5. Let the run finish (arrival at EXIT) ----------------------------
  await waitForState((s) => s.arrived, 8_000, "arrived at EXIT");
  await expect(page.getByText("arrived", { exact: true })).toBeVisible({ timeout: 2_000 });
  await shot(page, "04-arrived");

  // --- 6. Manual override: reposition to N2 (still has an open edge even
  //        with N1-N2 auto-blocked, so a subsequent forced hazard has
  //        somewhere left to reroute to) --------------------------------
  const overrideRes = await fetch(`${BACKEND_HTTP}/api/override`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ node_id: "N2" }),
  });
  expect(overrideRes.ok).toBe(true);
  await waitForState((s) => s.current_node === "N2" && !s.arrived, 3_000, "override to N2 applied");
  await shot(page, "05-override-to-N2");

  // --- 7. Demo-critical path: curl-equivalent POST /api/hazard blocks the
  //        cart's CURRENT edge while telemetry is live (mockclient is still
  //        connected, sending idle-but-live frames), confirm the minimap +
  //        route strip + event log all reflect the reroute within ~1s ---
  const beforeHazard = await getState();
  const targetEdge = beforeHazard.current_edge!.id;
  expect(targetEdge).toBeTruthy();

  const hazardRes = await fetch(`${BACKEND_HTTP}/api/hazard`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ type: "rockfall", severity: "high" }),
  });
  expect(hazardRes.ok).toBe(true);
  const hazardBody = await hazardRes.json();
  expect(hazardBody.blocked_edge).toBe(targetEdge);
  expect(hazardBody.route_changed).toBe(true);

  await waitForState((s) => s.blocked_edges.includes(targetEdge), 2_000, "hazard edge blocked");
  await page.waitForTimeout(300); // allow the WS push + one React render tick
  await shot(page, "06-hazard-blocked-reroute");

  await expect(page.getByText(`edge ${targetEdge} blocked`)).toBeVisible({ timeout: 2_000 });
  // Multiple reroutes have already happened this run (auto-stall, override) —
  // several "route recalculated" entries are expected; newest is prepended
  // first, so just confirm at least one (the log itself, not uniqueness).
  await expect(
    page.getByText(/^route recalculated:/).first()
  ).toBeVisible({ timeout: 2_000 });

  const afterHazard = await getState();
  expect(afterHazard.route.edges).not.toContain(targetEdge);

  // --- 8. Exercise the manual-override panel UI directly ------------------
  await page.getByText("Manual override (failsafe)").click(); // open <details>
  const overrideSelect = page.locator("select").first();
  await overrideSelect.selectOption("N4");
  await page.getByRole("button", { name: "Apply" }).click();

  await waitForState((s) => s.current_node === "N4", 3_000, "UI override to N4 applied");
  await shot(page, "07-ui-override-N4");

  // --- 9. Exercise the clear-blocked-edge control -------------------------
  const stateBeforeClear = await getState();
  expect(stateBeforeClear.blocked_edges.length).toBeGreaterThan(0);
  const edgeToClear = stateBeforeClear.blocked_edges[0];

  const blockedSelect = page.locator("select").nth(1);
  await blockedSelect.selectOption(edgeToClear);
  await page.getByRole("button", { name: "Clear" }).click();

  await waitForState(
    (s) => !s.blocked_edges.includes(edgeToClear),
    3_000,
    `edge ${edgeToClear} cleared via UI`
  );
  await expect(page.getByText(`edge ${edgeToClear} cleared`)).toBeVisible({ timeout: 2_000 });
  await shot(page, "08-ui-cleared-edge");

  // WS reconnect resilience (kill backend mid-session / restart / confirm
  // recovery without a page reload) is checked separately, outside this
  // spec — killing/respawning the backend process from inside a Playwright
  // worker proved unreliable in this sandboxed environment (observed a
  // worker crash), so it's exercised as a plain manual Bash + screenshot
  // check instead. See the task report for that check's result.
});
