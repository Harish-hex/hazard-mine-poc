#!/usr/bin/env bash
# Starts the Go backend and the Next.js dashboard together for local dev.
# Ctrl-C stops both.
#
# Usage:
#   ./dev.sh           # backend (:8080) + dashboard (:3000)
#   ./dev.sh --mock    # also starts the fake-ESP32 mock telemetry client

set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

pids=()
cleanup() {
  echo
  echo "Stopping..."
  for pid in "${pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "Starting backend (go run ./cmd/server) on :8080..."
go run ./cmd/server &
pids+=($!)

if [[ "${1:-}" == "--mock" ]]; then
  # Give the backend a moment to be listening before the mock client dials in.
  sleep 1
  echo "Starting mock telemetry client (go run ./cmd/mockclient)..."
  go run ./cmd/mockclient &
  pids+=($!)
fi

echo "Starting dashboard (npm run dev) on :3000..."
(cd dashboard && npm run dev) &
pids+=($!)

wait
