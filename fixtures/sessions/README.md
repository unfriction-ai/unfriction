# Selected-turn acceptance fixtures

`codex.jsonl` and `claude.jsonl` are **synthetic composites**, not captured
full sessions. They reuse the reduced acquisition record shapes documented in
`../README.md`, with constructed identities and Claude parent links. All prompts,
paths, tool outputs, and session identifiers are invented. They specify the
selected-turn behavior but do not establish compatibility with complete vendor
recordings.

Both contain two user turns. The second prompt is `Go`; its explanatory context
comes from the first turn. The second turn has exactly one read of `selected.md`
and result `SELECTED_PAYLOAD`. `EARLIER_PAYLOAD` belongs exclusively to turn one.
The selected Codex request is on physical line 10; its result is on line 11.
The selected Claude request is on line 6; its result is on line 7. The Claude
second user message retains its parent in the preceding turn.

## Full-session evidence gap (2026-10-28)

A read-only structural survey of the eight most recently modified local files
under each vendor's session storage inspected record names, versions, counts,
and parent-link continuity. It did not copy prompts, paths, IDs, tool payloads,
or private recordings into this repository. The survey is exploratory and does
not claim representative population coverage or establish import semantics.

- Codex samples report 0.164.0 or 0.165.0. Six of eight files contain both
  `event_msg/item_completed` and raw `response_item/custom_tool_call` plus
  `custom_tool_call_output` records. They also contain `additional_tools`,
  `thread_settings_applied`, and `token_usage_record`; some contain `compacted`
  and `turn_aborted`. The other two contain session/settings metadata only.
  This survey does not prove which records duplicate an operation or how they
  should be correlated. The existing adapter rejects mixed representations.
- Claude samples report 2.1.304, 2.1.306, or 2.1.308. They include `mode`,
  `permission-mode`, `atis-latch`, API request bookkeeping, system records,
  and attachments beyond the existing reduced contract. Five files have more
  than one string user prompt; each of those has at least one message/hook
  parent that does not equal the immediately preceding retained node. This is
  not proof of semantic branching: repeated records, bookkeeping, and ancestry
  require investigation before defining safe traversal.

## Authorized contract extension

The user approved extending selected-session import to verified current recording
formats while preserving strict prepared-path validation. The revised OpenSpec
change supersedes the initial one-representation/immediate-linear-chain scope.
The composites above remain synthetic and do not satisfy the real-session
evidence task by themselves.

A focused follow-up structural check found:

- In the sampled active Codex 0.165.0 session, raw custom-call/result envelopes
  surrounded groups of structured operations; raw call IDs had no direct matches
  to structured item IDs in the sample. Do not assume these are duplicate events
  or join them by position. Producer semantics or evidenced identity relations
  are still required before admitting a representation rule.
- In the sampled Claude 2.1.308 session, including all UUID-bearing records
  resolved every parent that appeared missing when only user/assistant/attachment
  nodes were considered. Sibling shapes included assistant tool-use and user
  tool-result records. These findings support graph/association validation, not
  automatic acceptance of arbitrary branching.

Neither observation establishes complete import support. Task 1.2 must still
land structurally faithful sanitized evidence and a verified record-role matrix;
Codex reconciliation and Claude ancestry have explicit implementation/test tasks.
Never silently discard raw records, rewrite ancestry, or claim these composites
demonstrate full-session support.
