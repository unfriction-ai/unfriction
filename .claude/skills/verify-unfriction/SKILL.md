---
name: verify-unfriction
description: "Drive the real unfriction Go CLI/TUI the way a user does and capture proof: `unfriction inspect` on prepared Codex/Claude Code traces and selected session turns, and the interactive `unfriction select` / pathless `unfriction inspect` picker under a PTY against isolated synthetic recordings. Use to prove a change to inspect output, session import, recording discovery, or the picker works in the built binary (not just `go test`), or when asked to verify/run/screenshot unfriction."
---

# Verify unfriction

`unfriction` is a short-lived Go CLI. Nothing stays running: a verification run is
one built binary plus an isolated "world" of fake recordings, and every drive is a
separate process. The helpers live in `.claude/skills/verify-unfriction/scripts/`
and `ptydrive/` (a small Go PTY driver that uses the repo's own `creack/pty`
dependency). They need `go` and `git`, and nothing else: no tmux, expect, or Python.
The PTY driver supports macOS and Linux only, the same platforms as the picker.

All commands below run from the repo root. Pick one `RUN_ID` per run and reuse it:

```sh
RUN_ID=$(date +%Y%m%d-%H%M%S)
V=.claude/skills/verify-unfriction/scripts
E="${VERIFY_UNFRICTION_ROOT:-${TMPDIR:-/tmp}/verify-unfriction}/evidence/$RUN_ID"   # this run's evidence
```

Locations, where `$ROOT` is `${VERIFY_UNFRICTION_ROOT:-${TMPDIR:-/tmp}/verify-unfriction}`
resolved to a real path (on macOS that is `/private/var/folders/.../T/verify-unfriction`):

| Path | What | Removed by cleanup |
| --- | --- | --- |
| `$ROOT/scratch/$RUN_ID/bin/` | `unfriction` and `ptydrive` built from this checkout | yes |
| `$ROOT/scratch/$RUN_ID/world/repo` | Throwaway git repo, the *current worktree* | yes |
| `$ROOT/scratch/$RUN_ID/world/other worktree` | Second worktree of the same repo. The space in the name is deliberate. | yes |
| `$ROOT/scratch/$RUN_ID/world/{codex-home,claude-home,home}` | Isolated `CODEX_HOME`, `CLAUDE_CONFIG_DIR`, and `HOME` | yes |
| `$ROOT/evidence/$RUN_ID/<label>/` | Proof artifacts, one directory per drive | **no** |

## Isolation (read before driving `select`)

The picker reads `CODEX_HOME` and `CLAUDE_CONFIG_DIR`. If either is empty it falls
back to `~/.codex` or `~/.claude`, which hold the user's private recordings. Never run
the built binary's `select` or pathless `inspect` by hand. Always go through
`tui.sh` or `cli.sh`. Both set `HOME`, `CODEX_HOME`, and `CLAUDE_CONFIG_DIR` to the
run's world on every invocation. Recording discovery is keyed to the git worktree
of the cwd, so interactive drives run inside the synthetic repo by default, never
inside this checkout.

Runs with different `RUN_ID`s are independent and can run side by side. `launch.sh`
refuses to reuse an existing `RUN_ID`, and the helpers refuse to overwrite an
evidence label.

## Launch

```sh
$V/launch.sh "$RUN_ID"
```

This builds `go build -buildvcs=false -o .../bin/unfriction ./cmd/unfriction`, the
README command, and also builds `ptydrive`. Then it creates the git repo plus the
second worktree. It installs `fixtures/sessions/codex.jsonl` and `claude.jsonl` into both
recording homes as `current.jsonl` (cwd is the repo) and `other.jsonl` (cwd is the
other worktree). Every recorded `"/workspace/example"` is rewritten to the real
worktree path.

Ready means it prints `ready: run_id=<id>` followed by the binary, repo, worktree,
and evidence paths, and exits 0. A build failure exits non-zero with Go's error.
Fix the checkout first. There is no server to tear down: teardown is just
`cleanup.sh`.

## Doctor

```sh
$V/doctor.sh "$RUN_ID"     # read-only; exit 0 = worth driving
```

It checks the following: launch happened, the binary reports `unfriction dev`,
`ptydrive` is built, and the **build fingerprint matches the current source**. The
fingerprint covers HEAD plus uncommitted changes under `cmd/`, `internal/`,
`go.mod`, and `go.sum`. It also checks that the synthetic repo has 2 worktrees, that
there are 2 recordings per family, and that isolation points inside the world. A
`STALE build` line means that you edited code after launch. Clean up and relaunch
with a new `RUN_ID` rather than proving old code. Run doctor first whenever a drive
behaves unexpectedly.

## Drive

There are three helpers. Each writes to `$ROOT/evidence/$RUN_ID/<LABEL>/`. A label
uses only `[A-Za-z0-9._-]` and is unique per run.

**Non-interactive** (`inspect <path>` and error paths). The command runs in a new
session with **no controlling terminal** and stdin `/dev/null`, which is how scripts
and CI see it. It writes `command.txt stdout.txt stderr.txt exit.txt pid.txt` and
exits with unfriction's status:

```sh
$V/cli.sh "$RUN_ID" <LABEL> [-C repo|other|DIR] -- <unfriction args...>
# default cwd = this checkout, so fixtures/... paths resolve
$V/cli.sh "$RUN_ID" prepared-claude -- inspect fixtures/claude-acquisition.jsonl
diff "$ROOT/evidence/$RUN_ID/prepared-claude/stdout.txt" fixtures/claude-acquisition.txt
```

**Interactive** (`select`, pathless `inspect`). The command runs under a fresh
140x35 PTY that is its controlling terminal, because the picker opens `/dev/tty`.
The run's `bin/` comes first on `PATH`, so write `unfriction`, not a path. Steps run
in order:

```sh
$V/tui.sh "$RUN_ID" <LABEL> [-C repo|other|DIR] [steps...] -- unfriction <args...>
# default cwd = synthetic repo (current worktree)
#   -expect TEXT      wait (default 5s) until TEXT is anywhere in the transcript
#   -expect-new TEXT  wait until TEXT appears after the last send/key
#   -send TEXT        type literal text (Go escapes like \r \x1b are honored)
#   -key NAME         enter esc tab up down left right pgup pgdn home end ctrl-c ctrl-u
#   -want-exit N      require this exit status after the steps
#   -timeout D        per-wait timeout, e.g. 10s
```

It writes `command.txt steps.log transcript.raw screen.txt stdout.txt exit.txt
tty-restored.txt pid.txt`. It exits 0 only if every step and `-want-exit` passed.
On failure it prints `ptydrive: FAIL <step> ...`. If the process is still alive
after a failure, it kills the process group after the timeout and still writes all
evidence.

**Run an emitted command.** The command that `select` printed is executed with
`sh -c`, exactly as pasted, without a terminal and in the synthetic repo:

```sh
$V/emitted.sh "$RUN_ID" <LABEL> <SELECT_LABEL> [-C repo|other|DIR]
```

Stable handles come from the picker's own strings in `internal/selection/picker.go`.
Wait on them, never on sleeps:

| Handle | Meaning |
| --- | --- |
| `Discovery complete · N sessions · M skipped` | Loading finished. Wait for this before typing. |
| `Search: <query>` | Query echo. |
| `User: <prompt>` | Preview of the highlighted turn. |
| `Previous agent:` / `Earlier user:` | Explanatory context in the preview. |
| `> current worktree / ...` | Highlighted row. Rows read `<group> / <branch> / <family> <timestamp> / Turn i of n: <prompt>`. |
| `Full turn context` | Tab detail view. |
| `No search matches; Ctrl+U clears the search.` | Empty search state. |
| `no supported recordings found for this repository` | Empty discovery (exit 1). |
| `refresh and reselect` | Source changed after discovery. |

Synthetic data: each family has 2 sessions (the current worktree and `other
worktree`) with 2 turns each. Turn 1 is `Read earlier.md and plan validation.` and turn 2 is
`Go` (`codex:second` or `claude-code:second`). Typing `validation` matches turn 1 by
its prompt and turn 2 through the earlier context, so `validation` followed by
`-key down` highlights `User: Go` in `current.jsonl`.

The per-feature recipes are in [`features/README.md`](features/README.md). Use them.
They list every entry point and the end state that proves each one.

## Evidence

Keep everything under `$ROOT/evidence/$RUN_ID/`. Standards:

- **Real user path only.** Use the built binary, real flags, and real keystrokes in a
  real PTY. Do not call Go functions, construct test-only models, or use
  `go test` output as the proof. `go test ./...` and `go vet ./...` are
  complementary, not a substitute.
- **Action and result.** For TUI drives, `steps.log` records each keystroke and the
  handle that appeared, `transcript.raw` holds the terminal byte stream and
  `screen.txt` holds the same stream with ANSI codes stripped, and `stdout.txt` and
  `exit.txt` hold the outcome. For CLI drives, keep `command.txt`, `stdout.txt`,
  `stderr.txt`, and `exit.txt` together.
  `screen.txt` is not a rendered frame. Bubble Tea redraws only the lines that changed,
  so assert on the handles it contains, never on row order or the `> ` cursor. Prove
  which row was chosen through the emitted command or the report. The highlighted
  row right after `Discovery complete` is **not deterministic**. Reach a known row by
  search or with `-key up` before you press Enter.
- **Side effects and second views.** An accepted `select` only *prints* a command.
  Prove that the command works by running it with `emitted.sh` and checking the
  report (`Task:\nGo`, `SELECTED_PAYLOAD`, no `EARLIER_PAYLOAD`). Prove prepared-trace
  output by `diff` against the committed golden `fixtures/*.txt` where one exists.
  Every PTY drive must show `tty-restored.txt` = `true`. When it is `false`, the
  file includes the before/after `stty -g` lines.
- **Read-only claims.** Inspection must never write: check that `git status
  --porcelain` in this checkout and `find <world> -newer <evidence>/launch.env -type f`
  are unchanged when you claim that inspection has no side effects.
- **No real recordings.** The emitted command's path must sit under
  `$ROOT/scratch/$RUN_ID/world/`. A path under `~/.codex` or `~/.claude` means that
  isolation broke. Treat that as a failed run and do not paste its content anywhere.

## Cleanup

```sh
$V/cleanup.sh "$RUN_ID"
```

Cleanup kills only process groups whose PIDs this run's `ptydrive` recorded in
`evidence/*/pid.txt`. It does so only while the live process still looks like this run's (args
contain `ptydrive-host`, the run's `bin/`, or its scratch dir). It never kills by process
name. Then it deletes `$ROOT/scratch/$RUN_ID`, including the synthetic repo and its
worktree, which live entirely inside scratch, so the real repo's
`git worktree list` is untouched. It prints `evidence kept: <path> (<n> files)`.
Run cleanup after every run, including failed iterations. To discard evidence,
delete `$ROOT/evidence/$RUN_ID` yourself. Cleanup never does that.

## Helpers

| Helper | Invocation |
| --- | --- |
| `scripts/launch.sh` | `$V/launch.sh RUN_ID` |
| `scripts/doctor.sh` | `$V/doctor.sh RUN_ID` |
| `scripts/cli.sh` | `$V/cli.sh RUN_ID LABEL [-C repo\|other\|DIR] -- ARGS...` |
| `scripts/tui.sh` | `$V/tui.sh RUN_ID LABEL [-C ...] [steps...] -- unfriction ARGS...` |
| `scripts/emitted.sh` | `$V/emitted.sh RUN_ID LABEL SELECT_LABEL [-C ...]` |
| `scripts/cleanup.sh` | `$V/cleanup.sh RUN_ID` |
| `scripts/env.sh` | Sourced by the others to set paths. It is not run directly. |
| `ptydrive/main.go` | Built by `launch.sh`. Its usage is in the file header. Package path components that start with `.` are skipped by `./...`, so it never enters `go test ./...`, `go vet ./...`, or release builds. |

A complete example, which is the proof this skill was validated with:

```sh
RUN_ID=$(date +%Y%m%d-%H%M%S); V=.claude/skills/verify-unfriction/scripts
$V/launch.sh "$RUN_ID" && $V/doctor.sh "$RUN_ID"
$V/tui.sh "$RUN_ID" select-codex -expect "Discovery complete" -send validation \
  -expect "Search: validation" -key down -expect "User: Go" -key enter -want-exit 0 \
  -- unfriction select --format codex
$V/emitted.sh "$RUN_ID" select-codex-inspect select-codex
$V/cleanup.sh "$RUN_ID"
```
