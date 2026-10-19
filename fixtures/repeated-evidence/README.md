# Real repeated Codex sequences: structured completed items

These are sanitized reductions of two existing local Codex recordings, prepared
on 2026-10-19. They establish that real single-task read → modification → read →
modification sequences exist. The CLI supports the narrow
`event_msg/item_completed` representation described below. Each extract has a
reviewed `.txt` expected report and is exercised through CLI acceptance and
Homebrew smoke tests. Neither extract is a complete history.

| Extract | Recorded version | Selected task (UTC) | Original retained lines |
| --- | --- | --- | --- |
| `codex-0.163.0.jsonl` | `0.163.0` | 2026-10-17 09:12:51–09:41:21 | start 287; task 290; read 304; update 368; read 438; update 444; final 809; completion 813 |
| `codex-0.162.1.jsonl` | `0.162.1` | 2026-10-14 13:44:59–14:05:52 | start 267; task 270; read 275; update 359; read 404; update 412; final 430; completion 434 |

Both tasks have one `UserMessage`, one matching `task_started`/`task_complete`
turn ID, and a final-phase `AgentMessage` whose text matches the completion
record. The four retained operations have distinct item IDs and the same turn
ID. Their recorded start/completion timestamps establish that the four selected
operations did not overlap. Neither selected task contains recorded subagent
activity. These checks were performed against the private originals; those
originals are not checked in.

Each read is a literal `cat <path>` represented by `CommandExecution`, with
`command=["/bin/zsh", "-lc", "cat <path>"]`, source `unified_exec_startup`,
`status=completed`, `exit_code=0`, and text `aggregated_output`. Each modification
is a `FileChange` with one `update`, a unified diff, `move_path=null`,
`status=completed`, stdout, and empty stderr. These status fields describe tool
feedback; they do not verify the task outcome.

The first task contains 81 CommandExecution and 27 FileChange records; the second
contains 30 and 7 respectively. Only two of each kind are retained per extract.
Other operations, outer JavaScript tool wrappers and their results, duplicate
response messages, commentary, reasoning, extensions, token information,
environment/session metadata, navigation fields, and timing metadata were
omitted. The first task's skill content block and text-element annotations are
also omitted when replacing the task text; the replacement has an empty
`text_elements` array. The extracts are
not complete inventories and do not explain why the agent chose the later work.

Sanitization replaces task/final text, thread/turn/item/process IDs, paths, working
directory, file contents, stdout, duration values, and diff contents/offsets.
All four selected targets are distinct in each original; this relationship is
preserved as `CONTEXT-1.md`, `PROJECT-1.md`, `CONTEXT-2.md`, and `PROJECT-2.md`.
The update hunk counts are preserved: two then one in `0.163.0`, four then three
in `0.162.1`. The replacement text and diffs are examples, not original content.
Record types, field value types, retained source order, statuses, and the
association of each operation with its feedback in the same item are preserved.
Only allowlisted fields are written to these extracts.

## Why the earlier search missed them

The earlier search inspected direct `response_item/function_call` and
`response_item/custom_tool_call` records. A second pass checked all 298 locally
available active/archived rollout files, including file creations and matching
results independently of adjacency. No qualifying sequence was found in that
supported direct representation. Searching the structured `item_completed`
records then found these two tasks using the already recognized read command
and single-file modifications.

## Supported mapping and input contract

These are four combined operation/feedback records, **not eight separately
recorded request/result events**. The adapter maps each `CommandExecution` to an
acquisition/observation pair and each `FileChange` to an action/result pair. Both
events retain the same original item ID and source line. Local IDs remain
`a1`, `a2`, ... and `act1`, `act2`, ... without a fixed repetition count or path
deduplication. Each pair gets a coverage notice in the report explaining that
record order does not establish operation start order. No timestamps are invented
and JavaScript wrappers are not interpreted.

- Every structured item requires a nonblank ID, the active turn ID, and a
  nonblank thread ID consistent across the structured records. Item IDs must be
  unique across messages and operations. A second task, a second final response,
  an operation before the task or after the final response, and items after
  completion fail explicitly.
- `UserMessage` contains exactly one nonempty `text` block; `text_elements` must
  be absent or an empty array. `AgentMessage` contains exactly one nonempty `Text`
  block and `phase=final_answer`. Other content, commentary, and phases remain
  unsupported in this reduced structured representation.
- `CommandExecution.command` must be exactly `["/bin/zsh", "-lc", <read>]`, with
  `source=unified_exec_startup`. The read uses the existing literal `cat <path>`
  or `sed -n '1,220p' <path>` grammar. Command syntax is validated independently
  of `parsed_cmd`; metadata cannot turn an unsupported command into an acquisition.
  Only `status=completed` with integer `exit_code=0` is supported. Text
  `aggregated_output` is required and may be empty. `stdout`, `stderr`, and
  `formatted_output` must be absent or null, avoiding conflicting output sources.
- `FileChange` requires exactly one literal target with `type=update`, explicit
  `move_path=null`, and a nonempty supported unified diff. Bare numeric hunk
  headers, context/removal/addition lines, matching hunk lengths, and an actual
  change in each hunk are checked. Named headers, file headers, no-newline markers,
  other change kinds, renames, and multiple targets remain unsupported. Hunk
  validation never reads the target file or verifies whether the patch applies.
  Only `status=completed` with present string stdout and empty string stderr is
  supported. Empty stdout remains an empty recorded result. Alternate aggregated
  output, exit-code, or formatted-output fields must be absent or null.
- Missing/null required feedback is malformed input, not a pending request.
  Other status/exit combinations, including failed operations, currently fail
  explicitly pending evidence for those structured variants. Legacy tool-error
  and missing-feedback behavior is unchanged.
- Process IDs, working directories, parsed-command metadata, durations, and other
  uninterpreted metadata do not drive behavior or outcome claims. Unknown item
  kinds are rejected, rather than silently discarded.
- Legacy `user_message`/`agent_message` and tool request/result records cannot be
  mixed with structured items in either order, even with different IDs. This
  avoids ambiguous double representation; it does not deduplicate by path.
  Existing recognized metadata and duplicate `response_item` message/reasoning
  records retain their existing handling.
- A matching `task_complete` must still agree with the active turn and final text.
  Final text or completed tool items alone leave execution `INCOMPLETE`. Normal
  termination does not independently verify engineering success. Invalid input
  produces a source-line diagnostic and no partial report.

Synthetic unit variants cover three operation pairs, repeated paths, duplicates,
invalid lifecycle/mixed representations, malformed diffs, missing/ambiguous
feedback, and unsupported operations. They are regression tests, not additional
claims of recorded vendor variants.
