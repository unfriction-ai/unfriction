package acceptance

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the real publisher without credentials or remote mutations. A public
// source repository must reach publication; private destinations must not.
func TestPublishRepositoryVisibility(t *testing.T) {
	script, err := filepath.Abs("../.github/scripts/publish.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, sourcePrivate, destinationPrivate, head string
		wantOK                                        bool
	}{
		{"public source", "false", "false", "source-sha", true},
		{"private source", "true", "false", "source-sha", true},
		{"private destination", "false", "true", "source-sha", false},
		{"stale source", "false", "false", "newer-sha", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, text string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), mode); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(filepath.Join(dir, "dist"), 0755); err != nil {
				t.Fatal(err)
			}
			write("dist/checksums.txt", fmt.Sprintf("%x  unfriction.rb\n", sha256.Sum256([]byte("formula"))), 0644)
			write("dist/unfriction.rb", "formula", 0644)
			write("release-plan.json", `{"version":"0.5.3","notes":"Release"}`, 0644)
			write("gh", `#!/usr/bin/env bash
set -eu
case "$*" in
 "api repos/unfriction-ai/unfriction/git/ref/heads/main --jq .object.sha") echo "$TEST_HEAD" ;;
 "api repos/unfriction-ai/unfriction --jq .private") echo "$TEST_SOURCE_PRIVATE" ;;
 "api repos/unfriction-ai/unfriction-releases --jq .private"|"api repos/unfriction-ai/homebrew-tap --jq .private") echo "$TEST_DEST_PRIVATE" ;;
 "api --paginate repos/unfriction-ai/unfriction-releases/releases --jq .[]") : ;;
 "release create "*|"release upload "*|"release edit "*) echo "$*" >> "$TEST_MUTATIONS" ;;
 "release view v0.5.3 --repo unfriction-ai/unfriction-releases --json isDraft --jq .isDraft") echo true ;;
 "api repos/unfriction-ai/homebrew-tap/contents/Formula --jq "*) : ;;
 "api --method PUT repos/unfriction-ai/homebrew-tap/contents/Formula/unfriction.rb --input formula-update.json") echo "$*" >> "$TEST_MUTATIONS" ;;
 "api --paginate repos/unfriction-ai/unfriction/git/matching-refs/tags/v0.5.3") echo '[]' ;;
 "api --method POST repos/unfriction-ai/unfriction/git/refs -f ref=refs/tags/v0.5.3 -f sha=source-sha") echo "$*" >> "$TEST_MUTATIONS" ;;
 *) echo "Unexpected GitHub operation: $*" >&2; exit 99 ;;
esac
`, 0755)
			cmd := exec.Command("bash", script)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "GH_TOKEN=test", "SOURCE_TOKEN=test", "VERSION=0.5.3", "GITHUB_SHA=source-sha", "GITHUB_REPOSITORY=unfriction-ai/unfriction", "GITHUB_REF=refs/heads/main", "TEST_SOURCE_PRIVATE="+tc.sourcePrivate, "TEST_DEST_PRIVATE="+tc.destinationPrivate, "TEST_HEAD="+tc.head, "TEST_MUTATIONS="+filepath.Join(dir, "mutations"))
			output, err := cmd.CombinedOutput()
			if (err == nil) != tc.wantOK {
				t.Fatalf("publish success=%v, want %v: %v\n%s", err == nil, tc.wantOK, err, output)
			}
			mutations, _ := os.ReadFile(filepath.Join(dir, "mutations"))
			if !tc.wantOK && len(mutations) > 0 {
				t.Fatalf("blocked publisher mutated GitHub: %s", mutations)
			}
			if tc.wantOK && !strings.Contains(string(mutations), "ref=refs/tags/v0.5.3 -f sha=source-sha") {
				t.Fatalf("publisher did not finish source tagging: %s", mutations)
			}
		})
	}
}
