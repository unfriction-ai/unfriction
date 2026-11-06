# unfriction verification map

This directory is the maintained source for verifying unfriction's user-facing behavior. Read the index before you drive the binary, then use the matching feature file as the recipe. The helper commands and paths are defined in [`../SKILL.md`](../SKILL.md).

## Baseline preconditions

- Run from the repo root with `V=.claude/skills/verify-unfriction/scripts` and a fresh `RUN_ID`.
- `$V/launch.sh "$RUN_ID"` printed `ready: run_id=<id>`.
- `$V/doctor.sh "$RUN_ID"` exits 0. No `STALE build` line appears.
- The isolated world holds `codex-home` and `claude-home`. Each has `current.jsonl` (cwd is the synthetic repo) and `other.jsonl` (cwd is `other worktree`). Both are copies of `fixtures/sessions/{codex,claude}.jsonl` with the cwd rewritten.
- Never drive `select` or pathless `inspect` outside `tui.sh`. Outside `tui.sh`, the picker falls back to the user's real `~/.codex` and `~/.claude`.

## Driving conventions

- Prepared traces and explicit session paths go through `$V/cli.sh`, whose default cwd is the checkout, so `fixtures/...` resolves. These runs have no terminal.
- The picker (`select`, `inspect` without a path) goes through `$V/tui.sh`, whose default cwd is the synthetic repo. Use `-C other` to stand in the second worktree.
- Wait on the picker's literal handles listed in `SKILL.md`, such as `Discovery complete` and `User: Go`. Never use a fixed sleep.
- Use one evidence label per entry point, named `<feature-id>-<entry>`, for example `select-codex`.
- Treat every command as literal. Keep quoting and flags unchanged.

## Proof and skip reporting

- CLI proof is `command.txt`, `stdout.txt`, `stderr.txt`, and `exit.txt` from one label.
- TUI proof is `steps.log`, `screen.txt`, `stdout.txt`, `exit.txt`, and `tty-restored.txt` = `true`.
- When a committed golden `fixtures/*.txt` exists, prove the report by `diff` against it, not by eyeballing.
- An accepted `select` proves nothing until `emitted.sh` runs its printed command and the report shows the selected turn.
- Record the feature ID and entry point in the label. Report an unreachable entry point with the attempted command and its unmet precondition.
- Do not report a skipped entry point as verified through a different path.

## Feature entry contract

Each feature file starts with an H1 title and one paragraph that describes the user-visible behavior. It then uses exactly four H2 sections in this order.

1. `Sub-features` lists short IDs, with one line for each behavior.
2. `How to get to it (user POV)` lists every user entry point.
3. `Driving it with verify-unfriction` starts with `Preconditions:` and uses labeled bullets that pair each user action with an exact command and an observable result.
4. `Gotchas` lists traps that can waste or invalidate a verification run.

Keep implementation details out of the map. Name only user paths, stable handles, required state, commands, and observable proof.

## Features

- [Inspect a prepared trace](./inspect-prepared-trace.md) covers single-turn Codex and Claude Code reports, automatic detection, the `--format` override, and measurement sections.
- [Inspect a selected session turn](./inspect-session-turn.md) covers `--format ... --turn ... -- <session>` import with earlier context and provenance.
- [Select recorded work](./select-recorded-turn.md) covers the interactive picker that discovers recordings across worktrees, with search, navigation, details, accept, cancel, and empty states.
- [Inspect from the picker](./inspect-picker.md) covers `unfriction inspect` without a path: pick a turn, then get the report in one step.
- [Diagnostics and compatibility reports](./diagnostics.md) covers non-zero failures, the prefilled issue URL, and the no-terminal guidance.
