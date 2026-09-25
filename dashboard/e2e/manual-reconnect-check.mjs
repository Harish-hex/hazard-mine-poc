// Throwaway manual verification script (not part of the automated suite):
// confirms the dashboard WS client actually recovers a mid-session backend
// restart (kill -> "Reconnecting..." -> restart -> "Connected" resumes),
// AND that it properly re-syncs to the NEW backend's state rather than
// keeping the stale pre-kill snapshot forever. Run directly with
// `node e2e/manual-reconnect-check.mjs` while:
//   - the Next.js dev server is already up on :3000
//   - the Go backend is up on :8080, deliberately left in a state that will
//     look OBVIOUSLY different from a fresh restart (e.g. overridden to N4,
//     route "Checkpoint Bravo -> Exit Ramp") so a successful re-sync to the
//     fresh backend's default N1 route is unambiguous in the screenshots.
// This intentionally does NOT run under the Playwright test-runner's own
// worker/process supervision (a prior attempt to fold this into the spec
// file triggered an unrelated worker crash when killing/respawning the
// backend process from inside a managed Playwright worker).
import { chromium } from "@playwright/test";
import { execSync, spawn } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const browser = await chromium.launch();
try {
  const page = await browser.newPage();
  await page.goto("http://localhost:3000");

  await page.getByText("Connected", { exact: true }).waitFor({ timeout: 10_000 });
  await page.getByText("Checkpoint Bravo  →  Exit Ramp").waitFor({ timeout: 5_000 });
  console.log("[1/5] initial connect OK, showing pre-kill state (Checkpoint Bravo -> Exit Ramp)");
  await page.screenshot({ path: path.join(__dirname, "screenshots", "reconnect-0-connected.png") });

  console.log("killing backend (whatever is bound to :8080)...");
  execSync("lsof -ti:8080 | xargs kill -9 2>/dev/null; true", { shell: "/bin/sh" });

  await page.getByText("Reconnecting…", { exact: true }).waitFor({ timeout: 10_000 });
  console.log("[2/5] observed Reconnecting… after backend killed");
  await page.screenshot({ path: path.join(__dirname, "screenshots", "reconnect-1-reconnecting.png") });

  console.log("restarting backend (fresh in-memory state -> N1 default route)...");
  const repoRoot = path.resolve(__dirname, "..", "..");
  const server = spawn("go", ["run", "./cmd/server"], {
    cwd: repoRoot,
    stdio: "ignore",
    detached: true,
  });
  server.unref();

  await page.getByText("Connected", { exact: true }).waitFor({ timeout: 15_000 });
  console.log("[3/5] reconnected (Connected) without page reload");

  // The real assertion: did it re-sync to the NEW backend's fresh N1 route,
  // or is it still showing the stale pre-kill "Checkpoint Bravo -> Exit
  // Ramp" from before the drop?
  await page
    .getByText("Shaft Entrance  →  Checkpoint Alpha  →  Checkpoint Bravo  →  Exit Ramp")
    .first()
    .waitFor({ timeout: 5_000 });
  console.log("[4/5] re-synced to fresh backend state (N1 default route) — no stale data");
  await page.screenshot({ path: path.join(__dirname, "screenshots", "reconnect-2-reconnected.png") });

  console.log("[5/5] done");
} finally {
  await browser.close();
}
