#!/usr/bin/env sh

set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
LOG_DIR="$ROOT_DIR/.logs"

mkdir -p "$LOG_DIR"

find_go_bin() {
  if command -v go >/dev/null 2>&1; then
    command -v go
    return 0
  fi

  for candidate in /usr/local/go/bin/go "$HOME/go/bin/go" "$HOME/.local/go/bin/go"; do
    if [ -x "$candidate" ]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  return 1
}

GO_BIN="$(find_go_bin || true)"
if [ -z "$GO_BIN" ]; then
  echo "Error: go binary not found in PATH or common locations." >&2
  echo "Install Go or export PATH (ex: /usr/local/go/bin)." >&2
  exit 1
fi

PIDS=""

cleanup() {
  echo ""
  echo "Stopping services..."
  for pid in $PIDS; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
  echo "All services stopped."
}

trap cleanup INT TERM EXIT

start_service() {
  service_path="$1"
  service_name="$2"
  log_file="$LOG_DIR/$service_name.log"

  echo "Starting $service_name..."
  (
    cd "$ROOT_DIR"
    "$GO_BIN" run "$service_path"
  ) >"$log_file" 2>&1 &

  pid=$!
  PIDS="$PIDS $pid"
  echo "  - PID: $pid"
  echo "  - Logs: $log_file"

  # Fail fast on startup errors instead of printing false positives.
  sleep 1
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "  - ERROR: $service_name failed to start." >&2
    echo "    Last log lines:" >&2
    tail -n 20 "$log_file" >&2 || true
    exit 1
  fi
}

start_service "./services/auth-service" "auth-service"
start_service "./services/user-data-service" "user-data-service"
start_service "./services/championship-service" "championship-service"
start_service "./services/race-data-service" "race-data-service"
start_service "./services/ingestion-service" "ingestion-service"

echo ""
echo "All services launched."
echo "Press Ctrl+C to stop everything."
echo ""

wait
