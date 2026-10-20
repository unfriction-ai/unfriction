# Recorded acquisition timing evidence

Reviewed for #29 on 2026-10-20. Original private recordings were accessible; they
remain outside Git. Existing import boundaries are unchanged. This evidence is
about recorded operation intervals, not model inference or end-to-end task time.

| Supported variant | Original evidence inspected | Timing shape and scope | Attribution |
| --- | --- | --- | --- |
| Codex legacy 0.142.0-alpha.1 | M2 source task 2026-06-22 08:40:37–08:41:15 UTC; selected shell result at line 208 | `Wall time: 0.0000 seconds` in the execution envelope; decimal seconds at vendor display precision | Matching `call_id`, only the supported shell result envelope |
| Codex legacy 0.138.0-alpha.7 | M3 task 2026-06-11; read result at line 80 | Same shell envelope and recorded `0.0000 seconds` | Matching `call_id`; patch durations are outside acquisition scope |
| Codex structured 0.163.0 | Existing repeated-evidence source; read completions at lines 304 and 438 | `duration: {"secs":0,"nanos":71249250}` and `{"secs":0,"nanos":7292}` | Same `CommandExecution` item ID as acquisition and observation |
| Codex structured 0.162.1 | Existing repeated-evidence source; read completions at lines 275 and 404 | `duration: {"secs":0,"nanos":8959}` and `{"secs":0,"nanos":77791}` | Same completed command item ID |
| Claude Code 2.1.302 | M2 original Read request/result at lines 222/223 | No operation-duration field in request, result, or `toolUseResult`; timestamps and model usage do not supply operation timing | Unavailable for this inspected shape; not a claim that all Claude versions lack telemetry |

## Scope verified against vendor implementation

Codex [0.163.0 command schema](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/protocol/src/items.rs)
uses Rust `Duration` for the optional command duration. The serialized shape uses
whole seconds and subsecond nanoseconds. The
[execution event mapper](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/core/src/tools/events.rs)
copies the execution output duration into the completed item using the call ID.
The [execution manager](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/core/src/unified_exec/process_manager.rs)
measures its output-collection/wait interval after process launch; its synchronous
path passes that interval to the completed event and tool response. The
[legacy manager](https://github.com/openai/codex/blob/rust-v0.142.0-alpha.1/codex-rs/core/src/unified_exec/process_manager.rs)
uses the same post-launch timer for its shell response.

Therefore these are attributable **recorded tool-execution durations**, specifically
vendor execution/output-wait intervals. They must not be described as the full
process lifetime, acquisition-decision latency, model-thinking time, or task elapsed
time. A displayed zero is recorded rounded evidence, not proof of instantaneous
execution. Do not derive timing by subtracting timestamps. Do not sum operation
durations: supported imports do not establish non-overlap or additive semantics.

## Reproducible sanitized evidence

`fixtures/timing-evidence/codex-0.163.0.jsonl` is the already-supported sanitized
repeated-evidence extract with only the two original duration objects restored.
All existing replacement task text, targets, IDs and content remain sanitized.
Unlike the older fixture's replacement 1000-nanosecond values, these two numeric
values are preserved from the actual source. Its request/result associations and
provenance are documented in `fixtures/repeated-evidence/README.md`.

`fixtures/claude-acquisition.jsonl` supplies the real supported missing-duration
case, checked against the accessible original. Missing optional timing is not zero.
A missing observation likewise has unavailable duration. Malformed optional timing
must not yield a numeric measurement; synthetic unit variants should cover null,
negative/noninteger fields, invalid nanosecond ranges, overflow, malformed decimal
text and duplicate envelope metadata. Those variants are parser robustness tests,
not claims of additional vendor evidence.

## Delivered attribution and acceptance boundary

The implementation reuses #26 request/observation associations and places optional
validated operation metadata on the corresponding observation. Each acquisition
reports recorded seconds (retaining nanosecond precision for structured values),
or `unavailable`, with the supporting source line. No total or second acquisition
counter is added. Public CLI acceptance covers the real structured fixture with
interleaved actions, legacy rounded zero, Codex/Claude missing metadata, and a
pending result. Focused tests cover malformed optional metadata, numeric overflow,
noninteger fields, invalid nanosecond ranges, and out-of-order ID association.
Malformed optional timing remains unavailable without rejecting previously
supported input. All earlier measurements and execution diagnostics are retained.
