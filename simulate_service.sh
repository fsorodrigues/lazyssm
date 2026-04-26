#!/usr/bin/env bash
# Simulates a long-running service that periodically outputs a timestamp and message.
# Usage: ./simulate_service.sh <message> [interval_seconds]

MESSAGE="${1:?Usage: $0 <message> [interval_seconds]}"
INTERVAL="${2:-1}"

trap 'echo "Shutting down..."; exit 0' SIGINT SIGTERM

while true; do
  echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") $MESSAGE"
  sleep "$INTERVAL"
done
