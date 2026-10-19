#!/usr/bin/env bash
set -euo pipefail
: "${VERSION:?expected version is required}"
export HOMEBREW_NO_AUTO_UPDATE=1
export HOMEBREW_NO_INSTALL_CLEANUP=1
# The installation must work without a GitHub token.
unset GH_TOKEN GITHUB_TOKEN HOMEBREW_GITHUB_API_TOKEN

if [[ "${1:-}" == local ]]; then
  (cd dist && shasum -a 256 -c checksums.txt)
  brew tap-new --no-git unfriction-ci/tap
  tap_dir=$(brew --repository unfriction-ci/tap)
  python3 - "$tap_dir/Formula/unfriction.rb" <<'PY'
from pathlib import Path
import sys
formula = Path('dist/unfriction.rb').read_text()
prefix = 'https://github.com/unfriction-ai/unfriction-releases/releases/download/v'
import re
formula = re.sub(re.escape(prefix) + r'[^/]+/', Path('dist').resolve().as_uri() + '/', formula)
Path(sys.argv[1]).write_text(formula)
PY
  formula=unfriction-ci/tap/unfriction
elif [[ "${1:-}" == public ]]; then
  formula=unfriction-ai/tap/unfriction
else
  echo 'usage: smoke-homebrew.sh local|public' >&2
  exit 1
fi
brew install "$formula"
brew test "$formula"
test "$(unfriction --version)" = "unfriction $VERSION"
unfriction inspect fixtures/first-run.jsonl > "$RUNNER_TEMP/inspect-actual.txt"
diff -u fixtures/first-run.txt "$RUNNER_TEMP/inspect-actual.txt"

unfriction inspect fixtures/codex-acquisition.jsonl > "$RUNNER_TEMP/codex-acquisition-actual.txt"
diff -u fixtures/acquisition.txt "$RUNNER_TEMP/codex-acquisition-actual.txt"
unfriction inspect --format claude-code fixtures/claude-acquisition.jsonl > "$RUNNER_TEMP/claude-acquisition-actual.txt"
diff -u fixtures/acquisition.txt "$RUNNER_TEMP/claude-acquisition-actual.txt"

unfriction inspect fixtures/codex-action.jsonl > "$RUNNER_TEMP/codex-action-actual.txt"
diff -u fixtures/codex-action.txt "$RUNNER_TEMP/codex-action-actual.txt"
unfriction inspect fixtures/claude-action.jsonl > "$RUNNER_TEMP/claude-action-actual.txt"
diff -u fixtures/claude-action.txt "$RUNNER_TEMP/claude-action-actual.txt"

for fixture in fixtures/tool-variants/*.jsonl; do
  fixture_name=$(basename "$fixture" .jsonl)
  unfriction inspect "$fixture" > "$RUNNER_TEMP/$fixture_name-actual.txt"
  diff -u "${fixture%.jsonl}.txt" "$RUNNER_TEMP/$fixture_name-actual.txt"
done
