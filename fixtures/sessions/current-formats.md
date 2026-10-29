# Current recording evidence

These are complete sanitized recordings, inspected on 2026-10-28, not reduced
traces or composites. `codex-current.jsonl` has 46 records from Codex 0.164.0;
`claude-current.jsonl` has 212 records from Claude Code 2.1.304. Each fixture line
maps one-to-one to the same physical line of its original recording. No records
were deleted, reordered, inserted, or joined. The originals remain private.
Other versions in the design are evidence targets, not claims established here.

## Sanitization

All non-allowlisted string values are replaced consistently with `redacted-N`.
The same original string always receives the same replacement within a file,
preserving identity equality, parent links, message/result references, and matching
completion text. Known record/block types, roles, phases, status values, tool names,
and the recorded version are retained. All other dictionary keys are consistently
renamed `field-N`, retaining nesting and cardinality but intentionally obscuring
unmodeled payload schemas. Numbers become zero except source ordinals, block
indices, and exit codes. Booleans, nulls, empty strings, and array order remain.

Timestamps, paths, commands, input programs, telemetry, and prose are placeholders.
These fixtures establish envelope/identity/ancestry behavior, not timestamp
parsing, arbitrary tool semantics, usage totals, or execution of redacted commands.
Do not infer an allowlist for an opaque nested payload from its renamed keys.
One-off selection/sanitization scripts live outside the source repository.

## Behavioral cases and source mapping

| Fixture | Selected identity | Original/fixture lines | Behavior |
| --- | --- | --- | --- |
| Codex | `redacted-1272` | start 38, user 41, agent 42, raw agent mirror 43, completion 46 | Second turn, no tools, duplicate message representation with identical agent ID; ancillary usage remains unmeasured for this version. |
| Claude | `redacted-14` | prompt 7, thinking block 26, text block 27, system nodes 36–37 | First user turn, no tools; two blocks share message/request identity but have distinct node UUIDs and block indices. An `end_turn` on a thinking block does not itself supply final text. |

Codex's first turn retains raw `exec` envelopes alongside structured command
records (16–30); they have distinct identities. Their redacted programs cannot
establish a raw-wrapper/operation mapping. They provide negative evidence against
joining by position and remain unsupported when selected until a separate rule
is verified. The selected second turn must not inherit that earlier tool activity.
Claude's later turns retain supported and unsupported tool names and all ancillary
records; their tool payloads are not normalized when selecting the first turn.

## Record-role and relationship matrix

This table defines the evidence available for implementing session-only rules;
it does not declare implementation complete. Prepared `Parse` retains its policy.

| Records | Required evidence | Session treatment | Negative case |
| --- | --- | --- | --- |
| Codex session/start/context/complete | session identity; unique turn IDs; completion matches active turn and final text | Establish boundaries and metadata; never count as acquisitions | Duplicate IDs, mismatched completion or context |
| Codex structured UserMessage/AgentMessage | thread/turn identity; typed text; final phase | Actual prompt/final; commentary is separate context | Conflicting raw mirror or repeated authoritative user/final |
| Codex raw assistant message mirror | Same nonempty message ID, owner turn and text as structured agent message | One message, retain both physical source locations | Different text for same ID; no identity-based association |
| Codex raw user message | Explicit turn attribution where present; structured user record is the turn prompt | Supplied prompt/context representation; no measured operation | Conflicting explicitly attributed prompt |
| Codex structured/raw tool records | Existing supported tool shape and independent recorded request identity | Normalize supported operations exactly once; no assumed wrapper join | Redacted/unsupported wrapper or ambiguous overlap |
| Codex settings, declarations, world state, reasoning, usage envelopes | Recognized record shape and session/turn scope where supplied | Explicit non-measured coverage; usage stays subject to existing verified-version limits | Unknown lifecycle record, conflicting turn ownership |
| Claude user prompt | UUID, session ID, explicit parent, role, string prompt | User turn boundary; ancestry determines context | Missing parent, duplicate identity, conflicting ownership |
| Claude assistant blocks | Node UUID/parent plus shared message ID/request ID and ordered block index | Preserve message identity and block provenance; final text comes from text blocks, not thinking | Conflicting identity/index/stop reason or ambiguous continuation |
| Claude tool results | `tool_use_id` and same owning user turn as request | Associate feedback even when not the adjacent node | Cross-turn or unmatched result |
| Claude system/attachment nodes | UUID, parent, session; recognized subtype | Preserve ancestry; explicit supplied-context/coverage notice | Cycle, missing ancestor, unknown subtype or sidechain |
| Claude last-prompt | Matching session and resolvable leaf UUID; optional string `lastPrompt` in current recordings | Validate navigation; never choose a convenient branch; prepared parser still rejects the four-field variant | Unresolved leaf, unknown field, or conflicting session |
| Claude mode/title/API-request/history/cost bookkeeping | Recognized envelope; matching session when present | Non-measured coverage; no invented task or completion | Unknown record class or conflicting session |

Synthetic regression tests separately cover abort disclosure, compaction with
missing boundaries, competing continuations (including behind system nodes),
correlated tool-result siblings, cross-turn reparenting, conflicting raw prompts,
foreign mirror ownership, and malformed optional version metadata. These tests
establish validation behavior; the no-tool current recordings do not establish
support for every operation in those versions. Non-ancestry bookkeeping near a
selected turn gets source-line coverage without claiming it belongs to that task.
