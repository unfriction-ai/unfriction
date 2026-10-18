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

## Input contract for M1

- UTF-8 JSONL with one object per nonblank line; maximum line size 4 MiB.
- One turn, identified by `event_msg.payload.type = task_started` and a nonempty
  `turn_id`, followed by one `user_message` with its task in `message`.
- The final response is an `agent_message` whose `phase` is `final_answer`.
  Commentary is not the final response.
- `task_complete` must carry the same `turn_id` and a `last_agent_message`
  matching the final response. This signals normal termination, not task success.
- Metadata and duplicate `response_item` messages are ignored. Tool-call response
  items are rejected; importing acquisitions is outside M1.
- Full multi-turn sessions must first be reduced to one turn. Missing required
  fields or unsupported lifecycle variants are not inferred from timestamps,
  assistant text, or vendor-specific heuristics.

This documents the observed rollout variant, not a guarantee of compatibility
with every Codex release. Future input variants belong in the Codex adapter;
the normalized run and report have no Codex field names.
