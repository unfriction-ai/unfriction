# First recorded run

`first-run.jsonl` is a sanitized, reduced extract of a real local Codex session
recorded on 2026-06-27. One contiguous turn was selected from the original JSONL
rollout, beginning with `task_started` and ending with `task_complete`. That turn
contained no tool-call response items. The original private recording is not
checked in.

Sanitization replaced the user's task and assistant response with
`Reply with the word ready.` and `ready`, consistently including the completion
event's duplicate response. Turn IDs and timestamps were replaced with fixed
values. Event order, event names, the `final_answer` phase, and the relevant JSON
structure come from the recording; this is a reduced real trace, not a claim
that the original user asked the replacement task.

Removed: turn context (paths, workspace details, model and permission metadata),
reasoning and encrypted content, token/rate-limit events, duplicate response
items, client identifiers, memory citations, and timing metadata. Only the four
task lifecycle events needed for this scenario remain. No private text or local
paths are included.

## Original M1 input contract

- UTF-8 JSONL with one object per nonblank line; maximum line size 4 MiB.
- One turn, identified by `event_msg.payload.type = task_started` and a nonempty
  `turn_id`, followed by one `user_message` with its task in `message`.
- The final response is an `agent_message` whose `phase` is `final_answer`.
  Commentary is not the final response.
- `task_complete` must carry the same `turn_id` and a `last_agent_message`
  matching the final response. This signals normal termination, not task success.
- Metadata and duplicate `response_item` messages are ignored. M1 rejected tool-call response
  items; M2 supports the additional variant documented below.
- Full multi-turn sessions must first be reduced to one turn. Missing required
  fields or unsupported lifecycle variants are not inferred from timestamps,
  assistant text, or vendor-specific heuristics.

This documents the observed rollout variant, not a guarantee of compatibility
with every Codex release. Future input variants belong in the Codex adapter;
the normalized run and report have no Codex field names.


## M2 acquisition fixture provenance

These are sanitized, reduced extracts of existing real local recordings, prepared
on 2026-10-19. They are not newly generated sessions with the displayed task.
Private originals are not checked in.

| Fixture | Source | Version recorded in source | Selected turn |
| --- | --- | --- | --- |
| `codex-acquisition.jsonl` | Codex rollout beginning 2026-06-20 | `0.142.0-alpha.1` (`cli_version`) | 2026-06-22 08:40:37–08:41:15 UTC |
| `claude-acquisition.jsonl` | Claude Code session JSONL | `2.1.302` (`version`) | 2026-10-17 11:52:44–11:52:52 UTC |

Codex: selected one `exec_command` call that performed a plain `cat` read and its
matching `function_call_output`, plus the original turn's `task_started`,
`user_message`, final-phase `agent_message`, and `task_complete`. The source turn
contained six tool calls; five other calls and their results were removed. This
fixture proves reconstruction of the retained pair, **not** a complete inventory
of that source turn or a causal explanation of its original final answer.

Claude Code: retained the task string, an assistant `Read` tool-use block, its
user `tool_result` block, and the final assistant text with `stop_reason=end_turn`.
This selected turn contained one tool call. The request's `tool_use` stop reason
and final `end_turn` stop reason were preserved.

For both fixtures, replaced task, file path, observation text, final response,
request IDs, timestamps, and session/turn identifiers with fixed example values.
Codex's duplicate final text in the completion record was replaced consistently.
Its output envelope structure was preserved while chunk ID and numeric metadata
were sanitized. Claude's Read input was reduced to `file_path`. The displayed
`# Example` is replacement observation text, not original file content. Retained
event order, event names, JSON value types, and request/result associations come
from the recordings. Optional fields, workspace details, model/usage information,
reasoning, attachments, API request records, and other metadata were removed.
Only sanitized allowlisted fields were written into the repository.

`acquisition.txt` is the expected report for both fixtures. Literal report equality
is convenient for these replacements; the cross-agent contract is behavioral
(task, acquisition, associated observation, final response, completion), not a
requirement that real agents produce identical text.

## M2 input contracts and limits

Both formats use UTF-8 JSONL, one object per nonblank line, at most 4 MiB per line.
Input order is authoritative; timestamps are not used to reorder events. Each
acquisition gets a deterministic local ID (`a1`, `a2`, ...); source request IDs and
line numbers remain in the model as provenance. Results may arrive out of request
order and are associated by ID. Multiple reads are accepted without modeling
modification actions or acting feedback loops.

### Codex (`--format codex`, the default)

The M1 lifecycle rules above still apply. In addition, between task and final:

- `response_item.payload.type=function_call`, `name=exec_command`, nonempty
  `call_id`, and JSON-string `arguments` containing `cmd=cat <path>`.
- The path is one unquoted literal using letters, digits, `_`, `.`, `/`, or `-`;
  it cannot begin with `-`. No options, multiple paths, substitutions, pipes,
  compound commands, or asynchronous process polling are supported. The report
  shows the requested path literally; it does not resolve it against `workdir`
  or inspect the filesystem.
- `function_call_output` must reference the same `call_id` and contain a string
  output envelope beginning `Chunk ID:`, with a `Process exited with code` line
  and `Output:` separator. The observation is the text following that separator,
  including an empty body. Execution metadata is not rendered as observation
  text; the process exit code is never used to infer run completion or success.
- Session metadata, turn context, response messages/reasoning, token counts, and
  agent reasoning are ignored. Other event forms are rejected explicitly.
  Newer Codex custom-tool/orchestration formats are not supported by this variant.

### Claude Code (`--format claude-code`)

- One `user` event with a nonempty string `message.content` establishes the task.
- Message roles must match event types. Every message carries the same nonempty
  `sessionId`; sidechain events and additional tasks are rejected.
- An assistant `tool_use` block must be named `Read`, with a nonempty `id` and
  `input.file_path`. The target is preserved literally. Optional read range
  arguments do not change the target; the observation is the actual returned text.
- A user `tool_result` block references `tool_use_id` and supplies string content.
  Empty text is an observation; missing, null, or structured content is rejected.
  Error text is preserved as returned and does not prove engineering success.
- An assistant event with text and `stop_reason=end_turn` establishes final response
  and execution completion. Text without that marker does not prove completion;
  unterminated text is not promoted to a final response. `tool_use` stop reasons
  require a tool-use block; other nonempty stop reasons are unsupported.
- Assistant thinking blocks and top-level file-history snapshots, progress, and
  queue-operation records are ignored. Other records/blocks are rejected; reduce
  full session exports by removing unsupported metadata before importing.

### Diagnostics

Duplicate request IDs, duplicate results, orphan results, events outside the
acquisition phase, malformed fields, unsupported variants, and contradictory
completion records fail with nonzero exit and no report. A valid truncated trace
returns `INCOMPLETE`. A request without a result remains visible with
`Diagnostic: Observation missing for a1`; no observation is invented. If explicit
termination evidence is present, execution remains `COMPLETED` alongside that
diagnostic. Missing observations do not silently change execution history.
