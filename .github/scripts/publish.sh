#!/usr/bin/env bash
set -euo pipefail
: "${GH_TOKEN:?Set PUBLIC_RELEASE_TOKEN with contents-write access to the two public repositories}"
: "${SOURCE_TOKEN:?source repository token required}"
: "${VERSION:?release version required}"
: "${GITHUB_SHA:?source commit required}"
: "${GITHUB_REPOSITORY:?source repository required}"
[[ "$GITHUB_REPOSITORY" == unfriction-ai/unfriction ]]
[[ "${GITHUB_REF:-}" == refs/heads/main ]]
[[ "$VERSION" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]
distribution=unfriction-ai/unfriction-releases
tap=unfriction-ai/homebrew-tap
tag="v$VERSION"

# Old queued runs must never publish over a newer main head.
current=$(GH_TOKEN="$SOURCE_TOKEN" gh api "repos/$GITHUB_REPOSITORY/git/ref/heads/main" --jq .object.sha)
if [[ "$current" != "$GITHUB_SHA" ]]; then
  echo 'main has advanced; this run will not publish.'
  exit 1
fi
# Source visibility does not affect binary publication; destinations must be public.
for repo in "$distribution" "$tap"; do
  [[ "$(gh api "repos/$repo" --jq .private)" == false ]]
done
(cd dist && sha256sum -c checksums.txt)
test "$(jq -r .version release-plan.json)" = "$VERSION"

# List first so a network/permission failure is never mistaken for absence.
gh api --paginate "repos/$distribution/releases" --jq '.[]' | jq -s --arg tag "$tag" 'map(select(.tag_name == $tag))' > existing-release.json
exists=$(jq length existing-release.json)
marker="<!-- source-commit: $GITHUB_SHA -->"
if [[ "$exists" == 0 ]]; then
  jq -r .notes release-plan.json > public-notes.md
  printf '\n%s\n' "$marker" >> public-notes.md
  # The public tag points only to the public distribution repo's README history.
  gh release create "$tag" --repo "$distribution" --target main --draft --title "unfriction $VERSION" --notes-file public-notes.md
else
  # A failed release may be resumed only from its original source commit.
  jq -e --arg marker "$marker" '.[0].body | contains($marker)' existing-release.json > /dev/null
fi

draft=$(gh release view "$tag" --repo "$distribution" --json isDraft --jq .isDraft)
if [[ "$draft" == true ]]; then
  gh release upload "$tag" --repo "$distribution" --clobber \
    "dist/unfriction_${VERSION}_darwin_arm64.tar.gz" \
    "dist/unfriction_${VERSION}_darwin_amd64.tar.gz" dist/checksums.txt
  gh release edit "$tag" --repo "$distribution" --draft=false --latest
else
  # Never overwrite published assets. Reruns must match the original bytes.
  mkdir -p published
  gh release download "$tag" --repo "$distribution" --pattern checksums.txt --dir published
  cmp dist/checksums.txt published/checksums.txt
fi

gh api "repos/$tap/contents/Formula" --jq '.[] | select(.name == "unfriction.rb") | .sha' > formula-sha.txt
python3 - <<'PY'
import base64, json, os
from pathlib import Path
payload = {'message': 'chore: release unfriction ' + os.environ['VERSION'],
           'content': base64.b64encode(Path('dist/unfriction.rb').read_bytes()).decode()}
sha = Path('formula-sha.txt').read_text().strip()
if sha:
    payload['sha'] = sha
Path('formula-update.json').write_text(json.dumps(payload))
PY
gh api --method PUT "repos/$tap/contents/Formula/unfriction.rb" --input formula-update.json > /dev/null

# Mark the source only after the release and formula exist. No source is pushed
# to either public repository. Existing tags are verified, never moved.
GH_TOKEN="$SOURCE_TOKEN" gh api --paginate "repos/$GITHUB_REPOSITORY/git/matching-refs/tags/$tag" > source-tags.json
existing=$(jq -r --arg ref "refs/tags/$tag" '.[] | select(.ref == $ref) | .object.sha' source-tags.json)
if [[ -n "$existing" ]]; then
  [[ "$existing" == "$GITHUB_SHA" ]]
else
  GH_TOKEN="$SOURCE_TOKEN" gh api --method POST "repos/$GITHUB_REPOSITORY/git/refs" -f "ref=refs/tags/$tag" -f "sha=$GITHUB_SHA" > /dev/null
fi
