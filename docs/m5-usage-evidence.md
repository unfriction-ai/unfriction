# M5 model usage evidence gate (#30)

Investigated 2026-10-20 against accessible private originals of the existing M2
fixtures. Originals and disposable extraction scripts remain outside Git. The
checked-in earlier fixtures deliberately removed usage; their missing fields do
not imply vendor telemetry is absent. Issue #30 has no comments or linked
implementation PR as of this investigation.

## Evidence and scope matrix

| Recorded source | Fields / units | Verified scope and update semantics | Confidence / limits |
| --- | --- | --- | --- |
| Codex `0.142.0-alpha.1`, selected task 2026-06-22 08:40:37–08:41:15 UTC, original lines 211, 221, 225 | `event_msg/token_count.info.total_token_usage`: integer input, cached input, output, reasoning output, total tokens | Cumulative session snapshot; includes earlier tasks. Never sum snapshots or label this reduced task's total. | High: version-pinned implementation and 37 successive update identities in source; no resets observed in this source. |
| Codex `0.138.0-alpha.7`, M3 task 2026-06-11 18:42:52–18:48:12 UTC, original lines 57–229 | 26 nonnull `token_count.info` records, same five integer token categories; first usage line 71, last 228 | Cumulative/last-shaped objects present; original field shapes inspected | Present, not absent. Version-specific source update semantics not yet checked, so numeric reporting remains unverified for this release. |
| Codex `0.162.1`, structured task 2026-10-14 13:44:59–14:05:52 UTC, lines 267–434 | 20 nonnull token-count records, first 280, last 433; six token fields including `cache_write_input_tokens` | Cumulative/last-shaped objects present alongside structured completed items | Present, not absent. Version-specific source/cache-write semantics not yet checked; do not alias cache-write to cached-input. |
| Codex `0.163.0`, structured task 2026-10-17 09:12:51–09:41:21 UTC, lines 287–813 | 61 nonnull token-count records, first 299, last 812; six token fields including `cache_write_input_tokens` | Version-pinned `TokenUsageInfo` retains append/clone snapshot semantics; cache-write is a separate accumulated field | Common cumulative semantics verified; precise cache-write category overlap and minimal numeric fixture coverage remain unverified. Numeric support is not implied merely by field presence. |
| Same Codex source | `last_token_usage`, same five integer fields | Last model-response usage, also repeated if a snapshot is re-emitted without a new response. Not acquisition-specific. | High for field semantics; no response identity in these records, so do not sum or deduplicate last-response usage by numeric equality. |
| Claude Code `2.1.302`, selected task 2026-10-17 11:52:44–11:52:52 UTC, original lines 222 and 227 | `message.usage.input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`: integers | Model-message counters, keyed by `message.id`, not task/session totals. | High for these fields and identity; actual message IDs are replaced in fixtures. |
| Same Claude session, original lines 247–248, plus 51 multi-record message-ID groups | Identical `message.usage` on thinking/text or multiple tool blocks | Duplicate snapshots of the same model message, not independent charges. All 51 groups have identical usage. | High for identical snapshots; conflicting snapshots remain unsupported rather than guessed. |
| Same Claude source | Nested `cache_creation` TTL fields, `output_tokens_details`, `iterations`, server-tool and service metadata | Separate vendor categories, some overlapping the top-level counters | Present but outside minimal supported counter set. Never add nested iterations or thinking tokens to top-level output. |

Codex cached input is a subset of input. Claude cache creation and cache read
are separate input categories. They must keep distinct labels; a cross-vendor
"cached tokens" total would discard important semantics. Missing, null, malformed,
negative, fractional or overflowing values are not a reported zero.

## Source verification

The version-pinned [Codex TokenUsageInfo implementation](https://github.com/openai/codex/blob/rust-v0.142.0-alpha.1/codex-rs/protocol/src/protocol.rs#L2017)
uses `new_or_append` / `append_last_usage`: new usage is added to the cumulative
counter and copied into the last-response counter. With no new usage, existing
information is cloned. Its `TokenUsage::non_cached_input` subtracts cached input
from input, confirming category overlap. `fill_to_context_window` is a separate
synthetic saturation path: it sets only total tokens while component counters
are zero. A minimal reader must not silently treat inconsistent total/components
as verified reported model usage.

The inspected [Codex 0.163.0 definitions](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/protocol/src/protocol.rs#L2363) retain the same cumulative/last update algorithm and add a separately accumulated `cache_write_input_tokens`. No assumption equates that counter to cached input. Additional source-version checks listed above have not been performed; these are limitations of this investigation, not vendor telemetry absence.

The official [Anthropic SDK Message and Usage definitions](https://github.com/anthropics/anthropic-sdk-go/blob/543d266ce81a4487281e084b013fef807dbb5f18/message.go#L8770)
attach usage to a uniquely identified message. They explicitly distinguish input,
cache creation, and cache read; their sum is request input. `MessageDeltaUsage`
fields are cumulative within a streamed message, not additive deltas. The local
Claude records establish the specific duplication behavior; the SDK alone does
not prove how every Claude Code release records its stream.

## Implemented boundary

Numeric reporting is gated to Codex `0.142.0-alpha.1` (recorded session metadata)
and Claude Code `2.1.302` (assistant record version). Missing, malformed or other
versions keep usage unavailable while preserving import compatibility. Only
Codex token snapshots inside the active task lifecycle participate. Neither
vendor usage establishes task completion. Raw model-message IDs never enter the
normalized model or report; reports use local ordinal IDs.

The implementation preserves the existing reduced, single-task importer. It reports Codex's latest valid
cumulative snapshot with source line and explicit session scope that may include
prior tasks; do not derive a task delta from missing pre-task baselines. Keep the
five vendor counter labels, including their overlapping categories. Identical
snapshots replace provenance or are ignored, never added. Malformed individual counters remain unavailable while valid separate counters
can be displayed. The Codex total stays unavailable unless both input and output
are present and valid; inconsistent complete totals invalidate the sequence.
Unsupported aggregate/category shapes are unavailable without changing otherwise
valid task reconstruction. A decreasing cumulative update or inconsistent
component/total identity should invalidate the measured sequence rather than
produce a plausible-looking number.

For Claude, retain one usage entry per model-message ID and source line. Identical
repeats are one entry; conflicting repeats invalidate that message's usage. Keep
input, output, cache creation and cache read separate; no grand total is needed.
Missing identity cannot safely support model-message deduplication and should be
unavailable. Missing counters remain missing, including cache categories.

All reports must explain that these counters are vendor-reported model usage,
not acquisition allocation, pricing, or tokens inferred from returned bytes.
Absence remains unavailable. The fixture below does not establish a trustworthy
run-level input total, so that conditional issue scenario must not be relabeled
as proven run-level telemetry.

## Sanitized artifacts and acceptance coverage

`fixtures/usage-evidence/codex-usage.jsonl` reuses the existing sanitized M2
acquisition fixture, restores the original `session_meta.payload.cli_version`
value `0.142.0-alpha.1`, and restores actual numeric usage objects from original lines
211, 221 and 225 in source order. The read pair precedes line 211. Final response
precedes line 225; completion follows it. The latest input snapshot is 1,467,546;
it includes prior turns and omitted operations. Counters are unmodified; task,
paths, content, IDs and timestamps retain the earlier sanitized replacements.

`claude-usage.jsonl` restores the four actual counters and replaced message IDs
at the two original assistant records. Report separately: input 2/output 81/cache
creation 61,605/cache read 0, and input 2/output 225/cache creation 672/cache read
61,605. The zero cache-read count is actual reported zero, not absence.

`claude-duplicate-evidence.json` is evidence-only, not an import fixture: it retains
source lines, version, block types and exact four counters from real duplicate
records, with one replacement message ID. Their end-turn thinking record is not
supported by the current lifecycle importer, so importing it is deliberately not
part of this increment. Unit/acceptance mutations can test identical/conflicting
usage snapshots in an otherwise supported trace and must identify themselves as
synthetic regressions, not new vendor recordings.

Implementation began with failing public CLI assertions for both numeric fixtures
and their scope/attribution labels; the absent-usage no-tool regression passed.
The RED log stays outside Git. Duplicate, partial, and unknown-version CLI
regressions were added after implementation. Focused parser tests cover invalid counts, missing/null fields, duplicate/conflicting message IDs,
Codex decreasing/inconsistent snapshots, cache separation, and recorded zero.
Existing byte measurements and acquisition/action reconstruction remain
unchanged. When no usage field is recorded, the report remains byte-identical and
has no model-usage section: this means unavailable evidence, never zero usage. No new all-session import or byte-to-token estimate is warranted.
