vu_die() { echo "verify-unfriction: $*" >&2; exit 2; }

[ -n "${1:-}" ] || vu_die "missing RUN_ID (pick one, e.g. RUN_ID=\$(date +%Y%m%d-%H%M%S))"
case "$1" in *[!A-Za-z0-9._-]*) vu_die "RUN_ID may contain only letters, digits, . _ -" ;; esac

RUN_ID=$1
VU_SKILL_DIR=$(cd "$(dirname "$0")/.." && pwd -P)
VU_SOURCE=$(cd "$VU_SKILL_DIR/../../.." && pwd -P)
VU_ROOT=${VERIFY_UNFRICTION_ROOT:-${TMPDIR:-/tmp}/verify-unfriction}
mkdir -p "$VU_ROOT"
VU_ROOT=$(cd "$VU_ROOT" && pwd -P) # real path: git reports real paths
VU_SCRATCH=$VU_ROOT/scratch/$RUN_ID
VU_EVIDENCE=$VU_ROOT/evidence/$RUN_ID
VU_BIN=$VU_SCRATCH/bin
VU_WORLD=$VU_SCRATCH/world
VU_REPO=$VU_WORLD/repo
VU_OTHER_WORKTREE_WITH_SPACE="$VU_WORLD/other worktree"
VU_CODEX_HOME=$VU_WORLD/codex-home
VU_CLAUDE_HOME=$VU_WORLD/claude-home
VU_HOME=$VU_WORLD/home

vu_isolated_env() {
  printf '%s\n' "HOME=$VU_HOME" "CODEX_HOME=$VU_CODEX_HOME" "CLAUDE_CONFIG_DIR=$VU_CLAUDE_HOME"
}

vu_source_fingerprint() {
  (cd "$VU_SOURCE" && {
    git rev-parse HEAD
    git status --porcelain -- cmd internal go.mod go.sum
    git diff HEAD -- cmd internal go.mod go.sum
  } | shasum | cut -c1-12)
}
