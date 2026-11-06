#!/bin/sh
set -u
. "$(dirname "$0")/env.sh" "${1:-}"
label=${2:-}; [ -n "$label" ] || vu_die "missing LABEL"
case "$label" in *[!A-Za-z0-9._-]*) vu_die "LABEL may contain only letters, digits, . _ -" ;; esac
shift 2
cwd=$VU_REPO
if [ "${1:-}" = "-C" ]; then
  case "$2" in repo) cwd=$VU_REPO ;; other) cwd=$VU_OTHER_WORKTREE_WITH_SPACE ;; *) cwd=$2 ;; esac
  shift 2
fi
[ -x "$VU_BIN/ptydrive" ] || vu_die "not launched; run launch.sh $RUN_ID"
[ ! -e "$VU_EVIDENCE/$label" ] || vu_die "evidence label already used: $VU_EVIDENCE/$label"

PATH="$VU_BIN:$PATH" exec "$VU_BIN/ptydrive" -out "$VU_EVIDENCE/$label" -dir "$cwd" \
  -env "HOME=$VU_HOME" -env "CODEX_HOME=$VU_CODEX_HOME" -env "CLAUDE_CONFIG_DIR=$VU_CLAUDE_HOME" \
  "$@"
