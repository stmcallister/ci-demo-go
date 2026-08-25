#!/usr/bin/env bash
#
# Dispatch paired runs of both CI workflows to build up run history.
#
# Usage:
#   ./scripts/dispatch-runs.sh [N]      # N pairs, default 3
#   DELAY=120 ./scripts/dispatch-runs.sh 4
#
# IMPORTANT: do not fire all 15-20 runs at once. Spread them across several
# sessions over a few days so the sample reflects a range of cache states,
# runner assignments, and times of day. Twenty runs in one burst is really
# one measurement repeated twenty times.
#
set -euo pipefail

RUNS="${1:-3}"
DELAY="${DELAY:-90}"
WORKFLOWS=("ci-no-cache.yml" "ci-cached.yml")

for i in $(seq 1 "$RUNS"); do
  for wf in "${WORKFLOWS[@]}"; do
    printf '[%s] dispatching %-18s (pair %s/%s)\n' "$(date +%H:%M:%S)" "$wf" "$i" "$RUNS"
    gh workflow run "$wf"
  done

  if [ "$i" -lt "$RUNS" ]; then
    echo "    sleeping ${DELAY}s so the runs don't stack up in the queue..."
    sleep "$DELAY"
  fi
done

echo
echo "Dispatched $((RUNS * 2)) runs. Current status:"
gh run list --limit $((RUNS * 2))
