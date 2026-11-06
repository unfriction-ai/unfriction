# Inspect a selected session turn

A user inspects one turn of a full multi-turn session by passing an explicit turn selector, for example `unfriction inspect --format codex --turn codex:second -- <session.jsonl>`. The report heads with `Selected turn:` provenance (source, session, turn position, previous agent, and earlier user context). It then reports only the selected turn's activity. Earlier turns' payloads never appear in the measured events.

## Sub-features

- `session-codex` selects a Codex turn by recorded turn ID (`codex:<turn-id>`).
- `session-claude` selects a Claude Code turn by user-message UUID (`claude-code:<uuid>`).
- `session-context` labels previous agent and earlier user text as explanatory and excludes them from measurements.
- `session-provenance` keeps the physical source lines of the selected records.
- `session-unknown-turn` rejects a selector that is not in the recording.
- `session-requires-path` rejects `--turn` with no path.

## How to get to it (user POV)

- Run `unfriction inspect --format codex --turn codex:<id> -- <path>`.
- Run `unfriction inspect --format claude-code --turn claude-code:<uuid> -- <path>`.
- Paste the command that `unfriction select` printed. That path is covered in `select-recorded-turn.md`.

## Driving it with verify-unfriction

Preconditions:

- `$V/doctor.sh "$RUN_ID"` exits 0, and `E` is set as in `SKILL.md`.
- The cwd is the checkout (the `cli.sh` default). Committed synthetic sessions are `fixtures/sessions/codex.jsonl` and `fixtures/sessions/claude.jsonl`. In each, turn `second` has the prompt `Go`.

- **Codex turn.** Run `$V/cli.sh "$RUN_ID" session-codex -- inspect --format codex --turn codex:second -- fixtures/sessions/codex.jsonl`. Exit `0`. Stdout starts `Selected turn:` with `Session: session-codex` and `Turn 2 of 2: second`. It contains `Task:` followed by `Go`, `Recorded event: selected-read (source line 10)`, and `SELECTED_PAYLOAD`. It does not contain `EARLIER_PAYLOAD`.
- **Claude turn.** Run `$V/cli.sh "$RUN_ID" session-claude -- inspect --format claude-code --turn claude-code:second -- fixtures/sessions/claude.jsonl`. Exit `0`. It shows `Session: session-claude`, `Turn 2 of 2: second`, source lines 6 and 7, and `SELECTED_PAYLOAD`. It has no `EARLIER_PAYLOAD`.
- **Context labeling.** In either stdout, `Previous agent: I will read selected.md to check validation.` and `Earlier user: Read earlier.md and plan validation.` appear above `Surrounding context is explanatory and excluded from selected-turn measurements.`
- **Unknown turn.** Run `$V/cli.sh "$RUN_ID" session-unknown-turn -- inspect --format codex --turn codex:nope -- fixtures/sessions/codex.jsonl`. It exits non-zero, stdout is empty, and stderr contains `turn "nope" not found; refresh or reselect the recording`.
- **Path required.** Run `$V/cli.sh "$RUN_ID" session-requires-path -- inspect --turn codex:second`. It exits `1`, and stderr is `Error: --turn requires a trace path; omit --turn to select interactively`.
- **Current-format sessions.** Run `$V/cli.sh "$RUN_ID" session-codex-current -- inspect --format codex --turn codex:<id> -- fixtures/sessions/codex-current.jsonl` with a turn ID taken from that file. Read the expected outcome (a report or an explicit unsupported-tool diagnostic) from `fixtures/sessions/current-formats.md` before you assert anything.
- **Proof.** Keep `stdout.txt` and `exit.txt` for both families, plus the absence check `! grep -q EARLIER_PAYLOAD "$E/session-codex/stdout.txt"`.

## Gotchas

- Display positions such as `Turn 2 of 2` are not selectors. Use the recorded ID with the family prefix.
- `--format` without `--turn` does not enable session import. The same session path then fails strict prepared-trace validation.
- Put `--` before the path. The path follows the flags.
- Inspection rereads and fingerprints the file. Do not edit a session while a drive that uses it is running.
