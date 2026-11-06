#!/bin/sh
set -u
. "$(dirname "$0")/env.sh" "${1:-}"

kill_process_group() {
  kill -TERM -- "-$1" 2>/dev/null || true
}

for f in "$VU_EVIDENCE"/*/pid.txt; do
  [ -f "$f" ] || continue
  pid=$(cat "$f")
  if kill -0 "$pid" 2>/dev/null; then
    args=$(ps -o args= -p "$pid" 2>/dev/null || true)
    case "$args" in
      *ptydrive-host*|*"$VU_BIN"*|*"$VU_SCRATCH"*)
        echo "stopping leftover process group $pid ($args)"
        kill_process_group "$pid" ;;
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
