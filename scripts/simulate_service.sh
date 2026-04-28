#!/usr/bin/env bash
# Simulates a long-running service that periodically outputs a timestamp and message.
# Usage: ./simulate_service.sh <message> [interval_seconds] [max_runtime_seconds]
#
# The optional max runtime is useful for testing how the app reacts when a
# managed process exits on its own, such as an inactivity timeout.
#
# When launched through the app, you can also set SIMULATE_SERVICE_MAX_RUNTIME_SECONDS
# in the parent environment to force the simulated service to terminate.

MESSAGE="${1:?Usage: $0 <message> [interval_seconds] [max_runtime_seconds]}"
INTERVAL="${2:-1}"
MAX_RUNTIME_SECONDS="${3:-${SIMULATE_SERVICE_MAX_RUNTIME_SECONDS:-0}}"

trap 'echo "Shutting down..."; exit 0' SIGINT SIGTERM

if ! [[ "$INTERVAL" =~ ^[0-9]+$ ]] || [[ "$INTERVAL" -lt 1 ]]; then
  echo "interval_seconds must be a positive integer" >&2
  exit 1
fi

if ! [[ "$MAX_RUNTIME_SECONDS" =~ ^[0-9]+$ ]]; then
  echo "max_runtime_seconds must be a non-negative integer" >&2
  exit 1
fi

START_TIME="$(date +%s)"

while true; do
  NOW="$(date +%s)"
  ELAPSED="$((NOW - START_TIME))"

  if [[ "$MAX_RUNTIME_SECONDS" -gt 0 ]] && [[ "$ELAPSED" -ge "$MAX_RUNTIME_SECONDS" ]]; then
    echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") $MESSAGE timed out after ${MAX_RUNTIME_SECONDS}s"
    exit 0
  fi

  echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") $MESSAGE"
  sleep "$INTERVAL"
done
