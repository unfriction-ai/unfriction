# Diagnostics and compatibility reports

When unfriction cannot produce a trustworthy report, it exits non-zero, writes nothing to stdout, and prints `Error: ...` on stderr. For detection or adapter failures, stderr also carries a version block and a prefilled public GitHub issue URL. The CLI makes no network request. Prompt text, paths, and tool output are never copied into the URL.

## Sub-features

- `diag-unknown-format` rejects a trace with no structural evidence and suggests `--format`.
- `diag-multi-turn` rejects a whole session passed without `--turn`, and reports the line number.
- `diag-compat-url` prints `Report a compatibility issue (review before submitting):` with a `github.com/unfriction-ai/unfriction-releases/issues/new?...` URL.
- `diag-no-terminal` makes `select` exit 2 with `requires a controlling terminal`.
- `diag-bad-flag` rejects an unsupported `--format` or `--output` value.

## How to get to it (user POV)

- Run `unfriction inspect <unsupported or malformed trace>`.
- Run `unfriction inspect fixtures/sessions/codex.jsonl` without `--turn`.
- Run `unfriction select` from a script or pipe without a terminal.
- Run `unfriction inspect --format nope <path>` or `unfriction select --output fish`.

## Driving it with verify-unfriction

Preconditions:

- `$V/doctor.sh "$RUN_ID"` exits 0, and `E` is set as in `SKILL.md`.
- For an ad hoc malformed trace, write it inside the run's scratch world, never in the checkout: `W=$(cd "$E/../../scratch/$RUN_ID/world" && pwd -P)`.

- **Unknown format.** Run `printf '{"foo":1}\n' > "$W/unknown.jsonl"`, then `$V/cli.sh "$RUN_ID" diag-unknown-format -- inspect "$W/unknown.jsonl"`. It exits `1` and `stdout.txt` is empty. `stderr.txt` starts `Error: unknown trace format: no distinctive structural evidence; specify --format codex or --format claude-code` and contains `Agent: unknown` and the issue URL.
- **Multi-turn without selector.** Run `$V/cli.sh "$RUN_ID" diag-multi-turn -- inspect fixtures/sessions/codex.jsonl`. It exits `1`, and stderr contains `line 8: multiple turns are not supported`, `Agent: Codex`, `Agent version: 0.163.0`, and the issue URL.
- **URL hygiene.** Check `grep -o 'https://github.com/[^ ]*' "$E/diag-multi-turn/stderr.txt"`. The output contains `Failure+category` and `Source+line%3A+8`. It contains none of `EARLIER_PAYLOAD`, `earlier.md`, or `/workspace`.
- **No terminal.** Run `$V/cli.sh "$RUN_ID" diag-no-terminal -C repo -- select`. It exits `2`, stdout is empty, and stderr is `Error: selection requires a controlling terminal; run interactively or supply a trace path: ...`.
- **Bad flag values.** Run `$V/cli.sh "$RUN_ID" diag-bad-format -- inspect --format nope fixtures/first-run.jsonl` and `$V/cli.sh "$RUN_ID" diag-bad-output -C repo -- select --output fish`. Each exits non-zero with `unsupported` on stderr and empty stdout.
- **Proof.** Keep `stderr.txt`, `stdout.txt` (empty), and `exit.txt` for each label.

## Gotchas

- `cli.sh` runs in a new session with no controlling terminal, so it reliably reproduces the no-terminal path even when the agent's own shell has a TTY.
- Do not open the issue URL or submit anything. Verifying the URL text is enough, and the CLI itself never contacts the network.
- Do not create malformed fixtures inside the checkout. Use the scratch world so that cleanup removes them.
- An explicit `--format` names the chosen adapter in the report draft. It does not prove which agent produced the trace.
