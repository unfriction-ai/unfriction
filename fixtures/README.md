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

## Trace-family selection

`unfriction inspect <file>` defaults to automatic detection (`--format auto`).
Explicit `--format codex` and `--format claude-code` bypass detection entirely;
only the chosen adapter validates the input. No adapter retries are performed.

The detector reads at most **65,536 bytes (64 KiB)** and examines at most **64
nonblank complete records** from that prefix. Blank lines consume bytes but not
records; malformed nonblank lines count toward the record budget. A trailing
unterminated line is eligible at EOF below the byte limit; a line cut by the byte
limit is not classified. The full bytes read, including unexamined records and
partial lines, are replayed before the remaining input to preserve the trace.

Structural evidence is deliberately narrow:

- Codex: top-level `type=event_msg` or `type=response_item` and an object `payload`.
- Claude Code: top-level `type=user` or `type=assistant`, an object `message`, and
  a nonempty string `sessionId`.

Metadata/housekeeping (such as `session_meta` or `file-history-snapshot`) supplies
no family evidence. Other unrecognized or malformed records also supply none;
they are not removed and remain subject to adapter validation. Filenames,
extensions, text content, and a common `type` field alone are never evidence.

All eligible records within the prefix are checked, even after a family is found.
Evidence for both families fails as ambiguous. No evidence fails as unknown, or
as prefix-limit exhaustion if either budget is reached. Those diagnostics suggest
an explicit `--format`. Read failures abort selection. Conflicting or invalid
records beyond the detection budget are still read and validated by the selected
adapter; detection does not promise that the complete input is supported.

Both M2 fixtures below and the original M1 fixture are detected automatically.
The existing adapter contracts and normalized reports remain unchanged.

## M2 input contracts and limits

Both formats use UTF-8 JSONL, one object per nonblank line, at most 4 MiB per line.
Input order is authoritative; timestamps are not used to reorder events. Each
acquisition gets a deterministic local ID (`a1`, `a2`, ...); source request IDs and
line numbers remain in the model as provenance. Results may arrive out of request
order and are associated by ID. Multiple reads are accepted without modeling
modification actions or acting feedback loops.

### Codex (`--format codex` override)

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
  agent reasoning are ignored. The recognized context records described below are accounted for separately.
  Other event forms are rejected explicitly.
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
  queue-operation records are ignored. The recognized session/context records
  described below are handled separately. Other records/blocks are rejected; reduce
  full session exports by removing unsupported metadata before importing.

### Diagnostics

Duplicate request IDs, duplicate results, orphan results, events outside the
acquisition phase, malformed fields, unsupported variants, and contradictory
completion records fail with nonzero exit and no report. A valid truncated trace
returns `INCOMPLETE`. A request without a result remains visible with
`Diagnostic: Observation missing for a1`; no observation is invented. If explicit
termination evidence is present, execution remains `COMPLETED` alongside that
diagnostic. Missing observations do not silently change execution history.


## Recognized session and context records

See [context-record regression fixtures and interpretation rules](context-records/README.md)
for world state, opaque compaction, session-start hooks, and constrained navigation
bookkeeping. Context-bearing records produce a separate `Context coverage` section
with source lines and explicit interpretation limits. They are never reported as
agent-selected acquisitions or ordinary read observations. This compatibility
increment does not make full multi-turn histories supported.

## M3 action fixture provenance

Prepared on 2026-10-19 from existing real local recordings. Private originals are
not checked in; these are reduced extracts, not newly generated runs of the
replacement task. No changes were replayed while preparing or inspecting them.

| Fixture | Recorded source/version | Selected turn (UTC) | Retained tool pairs |
| --- | --- | --- | --- |
| `codex-action.jsonl` | Codex rollout beginning 2026-06-11, `cli_version=0.138.0-alpha.7` | 2026-06-11 18:42:52–18:48:12 | `exec_command` range read; `apply_patch` custom-tool update |
| `claude-action.jsonl` | Claude Code session, `version=2.1.234` | 2026-08-18 13:12:38–13:13:52 | `Read`; `Edit` with `replace_all=false` |

Codex retains lifecycle records from source lines 57–229, the read request at
line 76 and its matching result at line 80, and the modification request at line
174 and its matching result at line 176. The source turn contained 32 tool calls;
30 calls and their results were omitted. The read used `sed -n '1,220p'` on one repository file; the patch
updated a different repository file. The replacement targets retain that
relationship (`GUIDE.md` and `README.md`). The source patch used `*** Begin Patch`,
one `*** Update File:`, one bare `@@` hunk, and `*** End Patch`. Its custom call
had `status=completed`; its output used the `Exit code` / `Wall time` / `Output`
envelope. Separate patch progress events, other tool calls and results, reasoning,
metadata, and intermediate commentary were removed. In particular, this extract
omits earlier file creation and subsequent edits: it is not a complete inventory
or a causal explanation of the original final answer.

Claude retains source lines 6, 18, 19, 33, 34, and 40: task, Read request/result,
Edit request/result, and final assistant response with `stop_reason=end_turn`.
The source turn contained five tool calls; three calls and their results were
omitted. The read and edit targeted the same file; the replacement target preserves that
relationship. The Read result explicitly carried `is_error=false`; the Edit
result omitted that field. Other tool calls/results, assistant text, reasoning,
usage, navigation identifiers, workspace metadata, and bookkeeping were removed.

Both fixtures replace task, paths, contents, final response, and request/session
IDs with fixed example values. Codex's duplicate completion text is replaced
consistently. Patch contents are replaced with a small heading replacement using
the observed single-hunk grammar; Claude old/new strings are replaced consistently.
Result text is sanitized replacement text, not the original output. Event shapes,
value types, retained source order, tool names, associations, and termination
markers come from the recordings. Timestamps and optional arguments are omitted.
Only allowlisted fields are retained; no private original content is included.

`codex-action.txt` and `claude-action.txt` are the expected reports. Their read
paths and result text intentionally differ; their acquisition/action/result
concepts are equivalent. Acceptance tests exercise both automatic detection and
explicit format overrides and confirm that target files remain unchanged.

## M3 additional input contracts

The M1/M2 contracts still apply. Inspection is read-only: it neither executes
commands nor opens, validates, or modifies the recorded target files. A recognized
file-modification request is an attempted action, not proof that it was needed,
applied, or correct. Broad tool-name-based intent classification is not supported.

### Codex additions

- The recorded read shape `exec_command` with `cmd=sed -n '1,220p' <path>` is
  accepted in addition to `cat <path>`. The range and quoting are literal; other
  ranges, flags, multiple paths, shell syntax, and command composition remain
  unsupported. This narrow addition allows the actual M3 recording to be used
  without rewriting its read mechanism into an invented `cat` call.
- `response_item.payload.type=custom_tool_call`, `name=apply_patch`, nonempty
  `call_id`, `status=completed`, and a string `input` containing one Add File or
  Update File patch. This call status is not execution termination evidence or a result.
- Patch target paths use the same literal-path character set as `cat`. Each hunk
  starts with bare `@@`, contains only context (` `), removal (`-`), and addition
  (`+`) lines, and includes at least one removal or addition. Add File bodies
  contain only `+` lines (at least one). Optional final newline is accepted.
  Whole-file deletes, renames, multiple targets, named hunk anchors, end-of-file
  markers, and other patch variants are unsupported.
- A matching `custom_tool_call_output` carries a string output starting with
  `Exit code: <integer>`, `Wall time: <number> seconds`, and `Output:` on separate
  lines. The body is preserved, including empty output. A nonzero exit code is
  preserved in the report and marked as a tool-reported error. Zero does not
  establish task success. Function-call and custom-call result kinds cannot be
  interchanged. The exact standalone `patch rejected by user` result is also
  preserved without inferring execution or task failure. Other unknown envelopes
  fail explicitly.

### Claude Code additions

- Assistant `tool_use` named `Edit`, with nonempty ID and `input.file_path`,
  nonempty string `old_string`, string `new_string` different from `old_string`,
  and explicit `replace_all=false`. Empty replacement text is allowed. Unknown
  Edit input fields, missing/null replacement fields, no-op replacements, and
  other modification tools remain unsupported.
- The observed alternative Edit input has exactly `file_path`, a nonnegative
  decimal string `offset`, and nonempty string `new_text`. The recording shows a
  tool-disabled error; this recognizes an attempt, not a proven editing API.
- A matching user `tool_result` carries string content or, for a Read, a single
  inline base64 PNG/JPEG image block. Images retain media type and decoded byte
  count as observations without interpreted content. Mixed/multiple blocks,
  unsupported media types, remote URLs, and malformed images fail explicitly.
  Optional Boolean `is_error=true` is rendered as tool-reported error evidence; it does not change
  execution state. An absent or false flag does not independently verify success.

### Association, reports, and limits

Actions receive local IDs `act1`, `act2`, ...; existing acquisitions retain `a1`,
`a2`, ... . Request IDs must be unique across both kinds. Every result is matched
to its originating request, while events remain in source order (including block
order within one Claude message). Original source IDs and line numbers remain in
the model. The report uses `Run events:` when actions exist; acquisition-only and
no-tool reports retain their previous text.

An action requires no prior acquisition. Missing action results produce
`Diagnostic: Result missing for act1`; explicit termination still produces
`COMPLETED`, and unfinished executions remain `INCOMPLETE`. Neither a final
response alone in Codex nor a tool result proves termination. Duplicate/unmatched
results, duplicate IDs, invalid lifecycle order, and unsupported variants fail
with source-line diagnostics and no partial report. Error fixtures derived by
mutating the sanitized recordings are synthetic negative tests, not additional
claims of observed vendor variants.

M3 does not evaluate patches, infer engineering outcomes, classify arbitrary shell
writes, or import full sessions. Its ordered event model also supports repeated
acquisitions and actions; the additional coverage below exercises that behavior
without introducing a loop model.

Additional real, sanitized recordings and precise limits for creation, line
deletion, user rejection, image reads, and offset-shaped Edit attempts are in
[the observed tool-variant fixtures](tool-variants/README.md).

## M4 repeated-sequence evidence

`claude-repeated.jsonl` was prepared on 2026-10-19 from an existing real Claude
Code recording, version `2.1.229`. The selected single task ran on 2026-08-13
15:04:06–15:07:08 UTC. The private original is not checked in.

Retained source lines: task 201; Read request/result 221/222; Edit request/result
226/234; Read request/result 251/252; Edit request/result 254/256; final assistant
response 291 with `stop_reason=end_turn`. Each retained request has its original
matching result, and their source order is unchanged. Both reads have
`stop_reason=tool_use`, as do both edits. The retained results omit `is_error`;
absence does not independently establish success.

The original task contained 21 tool calls; 17 calls and their feedback were
omitted, along with other content blocks, intermediate assistant text, reasoning,
usage information, navigation IDs, timestamps, and workspace metadata. The first
read/edit pair targeted one file, and the second pair targeted another. This
relationship is preserved as `README.md` and `GUIDE.md`. Thus the fixture proves
repeated acquisition/action within one task, not rereading the same file or a
complete account of the original execution.

Task, final response, file contents, result text, paths, session ID, and tool IDs
are fixed replacement examples. Read inputs retain only `file_path`; edits retain
the observed replacement fields and `replace_all=false`, with sanitized strings.
The requests' `tool_use` and final `end_turn` stop reasons are preserved. Only
allowlisted fields are retained. The displayed task was not the original task,
and the reported update text is not independent verification of an edit.
`claude-repeated.txt` is the manually specified expected report.

The public CLI acceptance tests also construct **synthetic** three-repetition
sequences for both vendors from the M3 fixtures. They assign new request IDs and
use one repeated target to check that neither adapter deduplicates by path or
limits repetition to two. Error flags, missing feedback, and late duplicate
results are deliberate mutations, not additional real-recording evidence.
Focused tests cover interleaved requests/results, source lines, missing feedback,
duplicate IDs, orphan results, and unsupported requests. Existing and absent
target files are checked to establish read-only inspection.

No production changes were needed for the legacy repetition scenarios. The existing event stream
preserves every request and matching observation/result without imposing strict
alternation or requiring feedback before subsequent requests. Completion remains
independent of tool status and missing feedback. Single-task and supported-tool
limits remain unchanged.

The broader second search located two real repeated
Codex sequences in the newer structured `event_msg/item_completed` representation.
Their [sanitized extracts and provenance](repeated-evidence/README.md) retain
combined operation/feedback records without inventing legacy tool calls. The
adapter supports the narrow recorded forms documented there; both fixtures have
expected reports, CLI acceptance tests, and distribution smoke coverage. Each
combined record maps to a request/feedback pair sharing its source ID and line,
with a report notice distinguishing record order from operation start order.
