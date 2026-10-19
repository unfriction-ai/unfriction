# Release and distribution

The application source stays in the private `unfriction-ai/unfriction` repository.
Public binaries and checksums live in `unfriction-ai/unfriction-releases`.
The public formula lives in `unfriction-ai/homebrew-tap`.
Both public repositories have independent history; never mirror the source repo.
GitHub's generated source archives then contain only public distribution metadata.

## Semantic version policy

Use Conventional Commits for commits merged to `main`, including squash titles.
The Go release utility implements the semantic-release workflow without an npm
runtime or manually edited version files. It examines commits since the highest
reachable stable `vMAJOR.MINOR.PATCH` tag. Prerelease tags are not release baselines.

| Commit | Result after the initial release |
| --- | --- |
| `fix(parser): handle blank records` | Patch: `0.1.0` → `0.1.1` |
| `feat(cli): add a report format` | Minor: `0.1.1` → `0.2.0` |
| `feat(cli)!: replace input format` | Major: `0.2.0` → `1.0.0` |
| Conventional header plus `BREAKING CHANGE: ...` footer | Major |
| `docs:`, `test:`, `chore:`, ordinary refactors | No release |

The highest applicable bump wins. The first release-driving commit produces
**0.1.0**, even if it is marked breaking; documentation-only history produces no
initial release. After that, a breaking change graduates 0.x to 1.0.0. This is the
explicit experimental policy; we do not silently map breaking changes to minor.

The automated public changelog reports counts of features, fixes, and breaking
changes. It deliberately omits private commit messages, author details, and links.
A source commit identifier is recorded for retry ownership, without copying
source or Git history. Human-readable public descriptions can be added later.

## Verify before publishing

```sh
go test -v ./...
go vet ./...
go run ./cmd/release -mode plan
go run ./cmd/release -mode build -version 0.1.0 -out /tmp/unfriction-candidate
```

The output directory must not already exist. The build produces two reproducible
tar.gz archives containing only the executable, `checksums.txt`, and
`unfriction.rb`. Version information is embedded with `-X main.version=...`.
Development builds report `unfriction dev`. Policy tests never publish releases.
Artifact tests validate both Mach-O architectures, archive contents, checksums,
formula URLs, and reproducibility. Acceptance tests execute versioned binaries.

CI runs on pushes and pull requests. It builds candidates once and installs those
same archives through Homebrew on fresh Apple Silicon and Intel macOS runners.
Both installations execute `--version`, `brew test`, and report comparisons for
the M1 no-tool fixture, both M2 acquisition fixtures, and both M3 read/modification
fixtures, the repeated Claude read/modification fixture, and the observed
tool-variant fixtures. M3, repeated-sequence, and tool-variant smoke checks use
automatic format detection. Codex repetition has synthetic regression coverage;
the real repeated extracts require structured-record adapter support before
they can join these smoke checks.
The local smoke test rewrites only archive URLs to local files; release checksums
and the rest of the formula stay unchanged. Non-release commits use candidate
version `0.0.0`, which is never published.

Only pushes to `main`, with all test and macOS jobs passing, may publish. The
publish job downloads the tested artifacts instead of rebuilding them. Releases
are serialized; stale main runs stop rather than overwrite a newer main release.
PR jobs have no publishing credentials.

## One-time infrastructure setup

This setup is not complete until the public repositories and credential exist:

1. Create public `unfriction-ai/unfriction-releases` with a `main` branch containing
   only a public README explaining installation. Never push the private source
   branch or source tags to it.
2. Create public `unfriction-ai/homebrew-tap` with a `main` branch, public README,
   and `Formula/.gitkeep` so the first formula update can list that directory.
3. Store a fine-grained token as `PUBLIC_RELEASE_TOKEN` in the private source
   repository. Grant Contents read/write only to these two public repositories.
   Keep the secret out of pull requests and source files. The workflow's own
   `GITHUB_TOKEN` creates tags in the private source repository.
4. Merge using a Conventional Commit (for example,
   `feat: automate releases and Homebrew distribution`). Do not create an initial
   release tag manually; the workflow calculates `0.1.0`.

The public release starts as a draft and receives an explicit allowlist of the
two archives and checksums. The formula is then updated automatically, and the
private source commit receives its version tag. Fresh macOS jobs finally install
the published formula without authentication and repeat the smoke test.
Issue #3 remains incomplete until this actual public installation succeeds.

## Retry and troubleshooting

- Missing `PUBLIC_RELEASE_TOKEN`, wrong visibility, or missing repositories fails
  publication. Complete the setup and rerun the failed workflow; don't manually
  increment versions or tags.
- A rerun at an already tagged commit retains the same version. Published assets
  are never overwritten: checksums must match. Draft asset uploads can be retried.
- A partial release belonging to a different source commit fails explicitly.
  Repair/rerun the original release before merging another release-driving change;
  do not retarget an existing version to new code.
- If publication succeeded but formula/tag creation failed, rerun the original
  workflow to finish the remaining steps. A repeated formula update is harmless.
- If the public install fails, inspect the `public-install` job for download,
  checksum, architecture, version, or report errors. A green build alone does not
  close the delivery issue.
- A newer `main` head makes a queued old publication fail. Let the newer workflow
  release all changes since the last successful tag.
- Archives are not Apple Developer ID signed or notarized in this first delivery
  path. The native Homebrew smoke jobs must pass before claiming install support.

## Install, update, verify, and remove

These commands become available after the first public release is published:

```sh
brew install unfriction-ai/tap/unfriction
unfriction --version
brew test unfriction-ai/tap/unfriction
unfriction inspect /path/to/recorded-turn.jsonl

brew update
brew upgrade unfriction

brew uninstall unfriction
brew untap unfriction-ai/tap
```

Installation and updates need no GitHub token or private repository access. A
404 on a release download means the release asset is missing or not public;
authentication is not the workaround. Report the version and failing URL.
