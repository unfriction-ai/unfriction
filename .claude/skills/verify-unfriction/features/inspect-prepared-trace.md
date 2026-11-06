# Inspect a prepared trace

A user runs `unfriction inspect <trace.jsonl>` on a reduced single-turn Codex or Claude Code recording. The report shows the task, context acquisitions and observations, attempted file modifications and their results, the final response, the execution state, and measurement sections. Inspection only reads the trace. It never executes recorded commands.

## Sub-features

- `prepared-acquisition` reports a read request and its observation for both families.
- `prepared-action` reports an acquisition followed by an attempted modification under `Run events:`.
- `prepared-repeated` keeps repeated reads and actions, with local IDs `a1, a2, ...` and `act1, ...`.
- `prepared-no-tool` reports a run with no tool use (`first-run.jsonl`).
- `prepared-detect` selects the family automatically from structure, not from the filename.
- `prepared-format-override` bypasses detection with `--format codex` or `--format claude-code`.
- `prepared-measurements` appends acquisition counts, volume, targets, timing, and model usage when present.
- `prepared-incomplete` reports `INCOMPLETE` with exit 0 for a valid unfinished trace.

## How to get to it (user POV)

- Run `unfriction inspect <path>` in a terminal (auto detection).
- Run `unfriction inspect --format codex <path>` or `unfriction inspect --format claude-code <path>`.

## Driving it with verify-unfriction

Preconditions:

- `$V/doctor.sh "$RUN_ID"` exits 0.
- The cwd is the checkout. This is the `cli.sh` default, so `fixtures/` resolves.
- `E` is set as in `SKILL.md`: `E="${VERIFY_UNFRICTION_ROOT:-${TMPDIR:-/tmp}/verify-unfriction}/evidence/$RUN_ID"`.

- **Claude acquisition.** Inspect a Claude read. Run `$V/cli.sh "$RUN_ID" prepared-acquisition-claude -- inspect fixtures/claude-acquisition.jsonl`. Exit `0`, and `diff "$E/prepared-acquisition-claude/stdout.txt" fixtures/claude-acquisition.txt` is empty.
- **Codex acquisition.** Run `$V/cli.sh "$RUN_ID" prepared-acquisition-codex -- inspect fixtures/codex-acquisition.jsonl`. Exit `0`. `head -n "$(wc -l < fixtures/acquisition.txt)" "$E/prepared-acquisition-codex/stdout.txt" | diff - fixtures/acquisition.txt` is empty, and `Acquisition measurements:` follows.
- **Actions.** Run `$V/cli.sh "$RUN_ID" prepared-action-codex -- inspect fixtures/codex-action.jsonl` and `$V/cli.sh "$RUN_ID" prepared-action-claude -- inspect fixtures/claude-action.jsonl`. Each exits `0`, its stdout diffs clean against `fixtures/codex-action.txt` or `fixtures/claude-action.txt`, and it contains `Run events:`.
- **Repeated.** Run `$V/cli.sh "$RUN_ID" prepared-repeated-claude -- inspect fixtures/claude-repeated.jsonl`. Its stdout diffs clean against `fixtures/claude-repeated.txt`. Also run `$V/cli.sh "$RUN_ID" prepared-repeated-codex -- inspect fixtures/repeated-evidence/codex-0.163.0.jsonl`, which exits `0` and lists `a2` or `act2`.
- **No tool.** Run `$V/cli.sh "$RUN_ID" prepared-no-tool -- inspect fixtures/first-run.jsonl`. Its stdout diffs clean against `fixtures/first-run.txt`.
- **Format override.** Run `$V/cli.sh "$RUN_ID" prepared-format-override -- inspect --format claude-code fixtures/claude-acquisition.jsonl`. Its stdout is identical to `prepared-acquisition-claude`. Run the mismatch `$V/cli.sh "$RUN_ID" prepared-format-mismatch -- inspect --format claude-code fixtures/codex-acquisition.jsonl`. It exits non-zero, `stdout.txt` is empty, and `stderr.txt` starts with `Error:`. The adapter does not fall back to the other family.
- **Measurements.** Run `$V/cli.sh "$RUN_ID" prepared-usage-claude -- inspect fixtures/usage-evidence/claude-usage.jsonl` and `$V/cli.sh "$RUN_ID" prepared-timing-codex -- inspect fixtures/timing-evidence/codex-0.163.0.jsonl`. The usage stdout contains `Recorded model usage:` with `model-message snapshot` lines. The timing stdout contains `Recorded tool-execution duration:`.
- **Other variants.** Each `fixtures/tool-variants/*.jsonl` that has a sibling `.txt` must diff clean. Loop: `for f in fixtures/tool-variants/*.jsonl; do l=variant-$(basename "$f" .jsonl); $V/cli.sh "$RUN_ID" "$l" -- inspect "$f"; diff "$E/$l/stdout.txt" "${f%.jsonl}.txt"; done`.
- **Proof.** Keep each label's `command.txt`, `stdout.txt`, `stderr.txt`, and `exit.txt`, plus the empty `diff` output, as the artifacts.

## Gotchas

- `fixtures/codex-acquisition.jsonl` has no same-named golden file. `fixtures/acquisition.txt` is its expected report body, and the measurement sections follow it, so compare a prefix rather than the whole file.
- A whole multi-turn session passed without `--turn` must fail. Do not use `fixtures/sessions/*.jsonl` here. That path is covered in `diagnostics.md`.
- `COMPLETED` means termination evidence, not task success. Never report it as "the task passed".
- Stdout contains only the report. Diagnostics go to stderr. Assert both streams.
