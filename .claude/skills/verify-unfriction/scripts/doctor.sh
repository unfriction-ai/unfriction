#!/bin/sh
# Read-only health check for one verification run. Exit 0 = worth driving.
# Usage: doctor.sh RUN_ID
set -u
. "$(dirname "$0")/env.sh" "${1:-}"

bad=0
ok()   { echo "ok    $*"; }
nope() { echo "FAIL  $*"; bad=1; }

[ -f "$VU_SCRATCH/launch.env" ] && ok "launched: $VU_SCRATCH" || { nope "no launch for $RUN_ID (run launch.sh $RUN_ID)"; exit 1; }
[ -d "$VU_EVIDENCE" ] && ok "evidence dir: $VU_EVIDENCE" || nope "evidence dir missing: $VU_EVIDENCE"

v=$("$VU_BIN/unfriction" --version 2>&1) && [ "$v" = "unfriction dev" ] && ok "binary: $v" || nope "binary: '$v'"
[ -x "$VU_BIN/ptydrive" ] && ok "ptydrive built" || nope "ptydrive missing"

built=$(sed -n 's/^fingerprint=//p' "$VU_SCRATCH/launch.env")
now=$(vu_source_fingerprint)
[ "$built" = "$now" ] && ok "build matches source ($now)" || nope "STALE build: built from $built, source is now $now (relaunch with a new RUN_ID)"

n=$(git -C "$VU_REPO" worktree list --porcelain 2>/dev/null | grep -c '^worktree ')
[ "$n" = 2 ] && ok "synthetic repo has 2 worktrees" || nope "synthetic repo worktrees: $n"

c=$(find "$VU_CODEX_HOME/sessions" -name '*.jsonl' 2>/dev/null | wc -l | tr -d ' ')
l=$(find "$VU_CLAUDE_HOME/projects" -name '*.jsonl' 2>/dev/null | wc -l | tr -d ' ')
[ "$c" = 2 ] && [ "$l" = 2 ] && ok "isolated recordings: codex=$c claude=$l" || nope "isolated recordings: codex=$c claude=$l"

case "$VU_CODEX_HOME$VU_CLAUDE_HOME$VU_HOME" in
  *"$HOME/.codex"*|*"$HOME/.claude"*) nope "isolation points at real home" ;;
  *) ok "isolation: HOME/CODEX_HOME/CLAUDE_CONFIG_DIR under $VU_WORLD" ;;
esac

exit $bad
