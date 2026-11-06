#!/bin/sh
# Tear down one verification run: stop any process group this run started that
# is still alive, then delete the scratch dir (binaries, synthetic repo,
# worktree, recording homes). Evidence is never deleted.
# Usage: cleanup.sh RUN_ID
set -u
. "$(dirname "$0")/env.sh" "${1:-}"

# Only PIDs recorded by this run's ptydrive (each is a session/process-group
# leader), and only if the live process still looks like ours.
for f in "$VU_EVIDENCE"/*/pid.txt; do
  [ -f "$f" ] || continue
  pid=$(cat "$f")
  if kill -0 "$pid" 2>/dev/null; then
    args=$(ps -o args= -p "$pid" 2>/dev/null || true)
    case "$args" in
      *ptydrive-host*|*"$VU_BIN"*|*"$VU_SCRATCH"*)
        echo "stopping leftover process group $pid ($args)"
        kill -TERM -- "-$pid" 2>/dev/null || true ;;
      *) echo "pid $pid from $f is now an unrelated process; leaving it alone" ;;
    esac
  fi
done

if [ -d "$VU_SCRATCH" ]; then
  rm -rf "$VU_SCRATCH"
  echo "removed scratch: $VU_SCRATCH"
else
  echo "no scratch to remove: $VU_SCRATCH"
fi
rmdir "$VU_ROOT/scratch" 2>/dev/null || true

if [ -d "$VU_EVIDENCE" ]; then
  echo "evidence kept: $VU_EVIDENCE ($(find "$VU_EVIDENCE" -type f | wc -l | tr -d ' ') files)"
else
  echo "no evidence recorded for $RUN_ID"
fi
