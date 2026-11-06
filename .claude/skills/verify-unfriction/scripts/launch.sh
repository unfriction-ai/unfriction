#!/bin/sh
# Build unfriction + ptydrive into scratch and create an isolated recording world.
# Usage: launch.sh RUN_ID
set -eu
. "$(dirname "$0")/env.sh" "${1:-}"

[ ! -e "$VU_SCRATCH" ] || vu_die "scratch for RUN_ID $RUN_ID already exists: $VU_SCRATCH (pick a new RUN_ID or run cleanup.sh $RUN_ID)"
mkdir -p "$VU_BIN" "$VU_WORLD" "$VU_HOME" "$VU_EVIDENCE"

cd "$VU_SOURCE"
go build -buildvcs=false -o "$VU_BIN/unfriction" ./cmd/unfriction
go build -o "$VU_BIN/ptydrive" "$VU_SKILL_DIR/ptydrive/main.go"

# Two real worktrees of one throwaway repository.
git init -q -b main "$VU_REPO"
git -C "$VU_REPO" -c user.name=Verify -c user.email=verify@example.invalid commit -q --allow-empty -m verify
git -C "$VU_REPO" worktree add -q -b other "$VU_OTHER"

# Synthetic sessions (fixtures/sessions) with the recorded cwd rewritten to the
# synthetic worktrees. Directory/file names are not identity; cwd is.
install_session() { # family cwd name
  case "$1" in
    codex) src=codex.jsonl; dst="$VU_CODEX_HOME/sessions/2026/10/29/$3.jsonl" ;;
    claude-code) src=claude.jsonl; dst="$VU_CLAUDE_HOME/projects/encoded-name-is-not-identity/$3.jsonl" ;;
  esac
  case "$2" in *'|'*|*'"'*|*'\'*) vu_die "scratch path has characters unsafe for JSON/sed: $2" ;; esac
  mkdir -p "$(dirname "$dst")"
  sed "s|\"/workspace/example\"|\"$2\"|g" "$VU_SOURCE/fixtures/sessions/$src" > "$dst"
  chmod 600 "$dst"
}
for family in codex claude-code; do
  install_session "$family" "$VU_REPO" current
  install_session "$family" "$VU_OTHER" other
done

cat > "$VU_SCRATCH/launch.env" <<EOF
run_id=$RUN_ID
source=$VU_SOURCE
fingerprint=$(vu_source_fingerprint)
version=$("$VU_BIN/unfriction" --version)
EOF
cp "$VU_SCRATCH/launch.env" "$VU_EVIDENCE/launch.env"

echo "ready: run_id=$RUN_ID"
echo "  binary:    $VU_BIN/unfriction ($("$VU_BIN/unfriction" --version))"
echo "  ptydrive:  $VU_BIN/ptydrive"
echo "  repo:      $VU_REPO"
echo "  worktree:  $VU_OTHER"
echo "  evidence:  $VU_EVIDENCE"
