#!/bin/sh
# Run the built unfriction non-interactively: new session, no controlling
# terminal, stdin /dev/null, isolated HOME/CODEX_HOME/CLAUDE_CONFIG_DIR.
# Captures command.txt, stdout.txt, stderr.txt, exit.txt under the evidence dir.
# Usage: cli.sh RUN_ID LABEL [-C DIR|repo|other] -- UNFRICTION_ARGS...
#   default cwd is the unfriction checkout, so fixtures/... paths resolve.
#   -C repo / -C other select the synthetic worktrees.
# Exit status is unfriction's exit status (124 if killed after 10s).
set -u
. "$(dirname "$0")/env.sh" "${1:-}"
label=${2:-}; [ -n "$label" ] || vu_die "missing LABEL"
case "$label" in *[!A-Za-z0-9._-]*) vu_die "LABEL may contain only letters, digits, . _ -" ;; esac
shift 2
cwd=$VU_SOURCE
if [ "${1:-}" = "-C" ]; then
  case "$2" in repo) cwd=$VU_REPO ;; other) cwd=$VU_OTHER ;; *) cwd=$2 ;; esac
  shift 2
fi
[ "${1:-}" = "--" ] || vu_die "expected -- before unfriction arguments"
shift
[ -x "$VU_BIN/ptydrive" ] || vu_die "not launched; run launch.sh $RUN_ID"
[ ! -e "$VU_EVIDENCE/$label" ] || vu_die "evidence label already used: $VU_EVIDENCE/$label"

out=$VU_EVIDENCE/$label
"$VU_BIN/ptydrive" -notty -timeout 10s -out "$out" -dir "$cwd" \
  -env "HOME=$VU_HOME" -env "CODEX_HOME=$VU_CODEX_HOME" -env "CLAUDE_CONFIG_DIR=$VU_CLAUDE_HOME" \
  -- "$VU_BIN/unfriction" "$@"
code=$?
echo "exit=$code stdout=$(wc -l < "$out/stdout.txt" | tr -d ' ') lines stderr=$(wc -l < "$out/stderr.txt" | tr -d ' ') lines evidence=$out"
exit $code
