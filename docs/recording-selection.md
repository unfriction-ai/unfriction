# Find a recorded turn

From a Git worktree, run:

```sh
unfriction select
# Limit discovery to one recording family:
unfriction select --format claude-code
# Emit literal zsh command text:
unfriction select --output zsh
```

Type to search the repository/worktree path, recorded branch, session identity,
timestamps, user prompts, previous agent response, and available earlier user
prompts in the recorded ancestry. A “Go” turn can match an earlier validation
request without changing its actual prompt. The preview labels that context
separately. Up/Down moves through matches; Left/Right clears the search and moves
to the neighboring chronological turn in the selected session. Ctrl+U clears
the search. Match-centered snippets keep the matching words visible. Tab opens
the full wrapped context; Up/Down, Page Up/Page Down, and Home/End scroll it, and
Tab returns to the list. Enter accepts; Escape or Ctrl+C cancels.

Selection prints an explicit inspect command and does not run it. For example:

```sh
unfriction inspect --format 'codex' --turn 'codex:recorded-turn-id' -- '/absolute/path/session.jsonl'
```

The UI uses the controlling terminal (`/dev/tty`), so stdout contains only the
accepted command. Redirect stdout to capture it while using the picker. Default
`--output posix` and `--output zsh` quote each recording argument literally,
including quotes, newlines, dollar signs, and backticks. Bash/fish-specific output
modes and Tab integration are separate deliveries. No startup file is modified.

## Inspect immediately after selection

```sh
unfriction inspect
unfriction inspect --format claude-code
```

Omitting the path opens the same picker. Its action is **Enter inspect**: acceptance
closes the picker and invokes the existing inspection service once, using the
selected path, family, and recorded turn identity. The report goes to stdout.
Earlier context and source provenance match explicit selected-turn inspection.
No shell integration or startup-file changes are required.

Escape or Ctrl+C exits pathless inspection successfully (status 0) with no report;
standalone `select` still returns 130 on cancellation. Empty discovery remains
status 1. With no controlling terminal, pathless inspection returns status 2 and
guidance to provide a trace path. `--turn` requires a path; omit it when using the
picker. Explicit paths always bypass discovery and retain strict prepared-trace
validation unless a turn selector is supplied.

## Discovery and association

| Family | Storage root used by unfriction | Supported layout |
| --- | --- | --- |
| Codex | `CODEX_HOME`, otherwise `~/.codex` | `sessions/<year>/<month>/<day>/*.jsonl` |
| Claude Code | `CLAUDE_CONFIG_DIR`, otherwise `~/.claude` | `projects/<project-directory>/*.jsonl` |

These override variables configure unfriction's lookup roots. Directory names
and filenames never establish repository identity or a session timestamp.
Claude subagent directories are excluded: they are not top-level user sessions.
Other JSONL placements encountered under the scanned roots produce a layout
diagnostic. Symlinked source files and non-regular files are not imported;
configured root directories may resolve through symlinks. Missing roots are empty
sources; unreadable roots and invalid/partial recordings are disclosed separately.

Recorded `cwd` is matched to Git's current worktree inventory. Current-worktree
sessions come first, followed by other worktree/recorded-branch groups. Within a
group, sessions are newest first using their recorded start timestamps; missing
timestamps sort last with deterministic identity/path ties. Turns retain recorded
order, oldest first. Recorded branch and the worktree's present branch are shown
separately; a missing recorded branch remains “unknown branch”.

An existing unrelated repository, including a nested independent repository or
submodule, is excluded even when its branch name matches.
Missing or relative cwd, unavailable/deleted working directories, and otherwise
unconfirmed worktree associations appear at the end as **UNCERTAIN repository
candidates**, with their evidence. They are not attributed to the current
repository. Encoded Claude project directory names are not evidence of a match.

Source paths and pending counts appear while summaries load. Two workers parse
summaries by default (the internal worker setting is capped at four). Successful
sources remain usable when another source fails. The catalog uses the same vendor
boundary rules as selected-turn inspection, but does not normalize measured tool
activity. Summaries retain explanatory text in memory; detailed source records
are discarded after each summary. No persistent cache is created in this release.

## Trust and failures

Discoverability does not establish tool support. A valid turn summary can still
contain tools that inspection does not support. Run the emitted command to obtain
an inspection or an explicit unsupported-input diagnostic; see the
[current-format matrix](../fixtures/sessions/current-formats.md).

Acceptance re-reads and validates the selected recording, checks file identity,
and compares a content fingerprint. Changed, replaced, deleted, or truncated
sources require restarting selection. Inspection revalidates the current file
again. Partial trailing JSON is not trimmed to manufacture a complete recording.
Source files and Git repositories are only read; recorded commands are never run.
Terminal control characters are escaped for display without changing source text
or command arguments.

| Outcome | Exit status | stdout |
| --- | --- | --- |
| Accepted selection | 0 | One quoted command |
| Escape / Ctrl+C | 130 | Empty |
| Empty or failed discovery | 1 | Empty; diagnostic on stderr |
| No controlling terminal | 2 | Empty; guidance on stderr |

No search matches is a visible picker state, distinct from empty discovery.
Terminal settings are restored on normal acceptance, cancellation, and empty
results. Existing explicit-path inspection remains noninteractive. Tab adapters
and persistent cache/refresh are subsequent features.

## Evidence and verification

On 2026-10-29, the private originals behind the already committed sanitized
fixtures were checked without copying prompts, paths, or tool output into this
document:

| Evidence | Observed storage and metadata |
| --- | --- |
| Codex 0.164.0 | Under the default Codex home, `sessions/<year>/<month>/<day>/<file>.jsonl`; session metadata has `cwd`, `git`, `timestamp`, and `cli_version`. |
| Claude Code 2.1.304 | Under the default Claude home, `projects/<directory>/<file>.jsonl`; records carry `cwd`, `gitBranch`, `timestamp`, and `version`. |

The path placeholders preserve directory depth and role, not private names.
This establishes the supported locator shapes, not blanket compatibility with
all releases or tool variants. Synthetic acceptance tests construct these layouts
in isolated homes and two real Git worktrees, exercising both families and
confirmation-only prompts. Current sanitized sessions also verify summary loading
without normalizing their unsupported tools.

The shared terminal view uses pinned Bubble Tea v1.3.10 for input, rendering,
terminal restoration, and cancellation. Go PTY tests use creack/pty v1.1.24; no
external fuzzy finder or Python runtime is required. The tests verify command
execution round-trips, status codes, terminal restoration, changed-source rejection,
worker bounds/cancellation, chronology, ancestry-aware search, and literal shell
arguments. The PTY suite targets macOS and Linux; other platforms retain direct
inspection but are not claimed as supported picker environments.
