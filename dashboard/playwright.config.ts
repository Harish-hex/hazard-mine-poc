import { defineConfig } from "@playwright/test";

// The dashboard dev server, Go backend, and mock telemetry client are all
// expected to already be running (localhost:3000 / :8080) — this suite is
// a smoke/regression test driven against the real stack, not an isolated
// unit test, so it does not manage a webServer lifecycle itself.
export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  retries: 0,
  reporter: [["list"]],
  use: {
    baseURL: "http://localhost:3000",
    trace: "off",
    screenshot: "off",
  },
});
