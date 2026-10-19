# Real repeated Codex sequences: pending adapter support

These are sanitized reductions of two existing local Codex recordings, prepared
on 2026-10-19. They establish that real single-task read → modification → read →
modification sequences exist. **The current CLI does not support their
`event_msg/item_completed` representation.** They are evidence for the remaining
adapter work, not passing smoke fixtures or complete histories.

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
omitted. The first task's skill content block is also omitted. The extracts are
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

## Remaining implementation decision

These are four combined operation/feedback records, **not eight separately
recorded request/result events**. Do not rewrite them into invented legacy tool
calls just to make the current parser accept them. A narrow adapter extension
would need to recognize the task/final structured messages and the two operation
kinds, preserve their IDs and source lines, and explicitly define how a combined
record maps to acquisition/observation or action/result events. Completion must
still require the matching `task_complete`, and unsupported commands, change
kinds, ambiguous feedback, and mixed duplicate representations must fail
explicitly. This does not require interpreting JavaScript wrappers.

M4's real Codex evidence has now been located. Its end-to-end Codex acceptance
and distribution smoke coverage remain open until the representation is supported.
