# Inspect from the picker

`unfriction inspect` without a path opens the same picker as `select`, with the action label `Enter inspect`. Accepting closes the picker and prints the inspection report for the chosen turn to stdout in one step. Escape or Ctrl+C exits 0 with no report. Without a controlling terminal it exits 2 with guidance to pass a trace path.

## Sub-features

- `picker-inspect-accept` uses Enter to print the selected-turn report (exit 0).
- `picker-inspect-format` uses `--format codex|claude-code` to filter the picker.
- `picker-inspect-cancel` uses Escape or Ctrl+C to exit 0 with empty stdout. Standalone `select` exits 130 here instead.
- `picker-inspect-no-tty` exits 2 with guidance when no controlling terminal exists.

## How to get to it (user POV)

- Run `unfriction inspect` in a terminal inside a git worktree.
- Run `unfriction inspect --format claude-code` or `unfriction inspect --format codex`.

## Driving it with verify-unfriction

Preconditions:

- `$V/doctor.sh "$RUN_ID"` exits 0, and `E` is set as in `SKILL.md`.
- Drives run through `tui.sh` in the synthetic repo.
- `W=$(cd "$E/../../scratch/$RUN_ID/world" && pwd -P)` is the real path of the run's world.

- **Accept (Claude).** Run `$V/tui.sh "$RUN_ID" picker-inspect-claude -expect "Discovery complete" -send validation -expect "Search: validation" -key down -expect "User: Go" -expect "Enter inspect" -key enter -want-exit 0 -- unfriction inspect --format claude-code`. Stdout starts `Selected turn:` and names `claude-home/projects/encoded-name-is-not-identity/current.jsonl`. It contains `Task:` followed by `Go` and `SELECTED_PAYLOAD`, and has no `EARLIER_PAYLOAD`. `tty-restored.txt` is `true`.
- **Accept (Codex) through details.** Run `$V/tui.sh "$RUN_ID" picker-inspect-codex -expect "Discovery complete" -send validation -expect "Search: validation" -key down -expect "User: Go" -key tab -expect "Full turn context" -key enter -want-exit 0 -- unfriction inspect --format codex`. Stdout shows `Session: session-codex` and `Turn 2 of 2: second`.
- **Second view.** Compare with the explicit path. Run `$V/cli.sh "$RUN_ID" picker-inspect-codex-explicit -C repo -- inspect --format codex --turn codex:second -- "$W/codex-home/sessions/2026/10/29/current.jsonl"`. Its stdout equals `picker-inspect-codex/stdout.txt`.
- **Cancel.** Run `$V/tui.sh "$RUN_ID" picker-inspect-cancel -expect "Discovery complete" -key esc -want-exit 0 -- unfriction inspect`. `stdout.txt` is empty.
- **No terminal.** Run `$V/cli.sh "$RUN_ID" picker-inspect-no-tty -C repo -- inspect`. It exits `2`, stdout is empty, and stderr names supplying a trace path.
- **Proof.** Keep both TUI labels and the explicit-path label. The equal stdout is the second view.

## Gotchas

- The picker's report replaces stdout. The picker frames go to the terminal, so `stdout.txt` holds only the report.
- Cancel exits 0 here but 130 in `select`. Use the matching `-want-exit`.
- `--turn` without a path is an error, not a picker filter. See `inspect-session-turn.md`.
- Pass the world path as a real path (`W` from the preconditions). Otherwise the report's `Source:` line differs from the picker's, and the second view fails on that line alone.
