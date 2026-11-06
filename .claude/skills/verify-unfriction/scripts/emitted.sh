#!/bin/sh
set -u
. "$(dirname "$0")/env.sh" "${1:-}"
label=${2:-}; from=${3:-}
[ -n "$label" ] && [ -n "$from" ] || vu_die "usage: emitted.sh RUN_ID LABEL FROM_LABEL [-C repo|other|DIR]"
case "$label$from" in *[!A-Za-z0-9._-]*) vu_die "labels may contain only letters, digits, . _ -" ;; esac
cwd=$VU_REPO
if [ "${4:-}" = "-C" ]; then
  case "${5:-}" in repo) cwd=$VU_REPO ;; other) cwd=$VU_OTHER_WORKTREE_WITH_SPACE ;; *) cwd=${5:-} ;; esac
fi
src=$VU_EVIDENCE/$from/stdout.txt
[ -s "$src" ] || vu_die "no emitted command in $src"
[ ! -e "$VU_EVIDENCE/$label" ] || vu_die "evidence label already used: $VU_EVIDENCE/$label"

out=$VU_EVIDENCE/$label
PATH="$VU_BIN:$PATH" "$VU_BIN/ptydrive" -notty -timeout 10s -out "$out" -dir "$cwd" \
  -env "HOME=$VU_HOME" -env "CODEX_HOME=$VU_CODEX_HOME" -env "CLAUDE_CONFIG_DIR=$VU_CLAUDE_HOME" \
  -- sh -c "$(cat "$src")"
code=$?
cp "$src" "$out/emitted-command.txt"
echo "exit=$code stdout=$(wc -l < "$out/stdout.txt" | tr -d ' ') lines evidence=$out"
exit $code
