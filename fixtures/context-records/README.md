# Session/context regression examples

These four regression fixtures combine one sanitized real record shape with the
already documented M2 task/read/result/completion fixture. They are **composites**,
not claims that these exact records and replacement task occurred together in an
original session. Existing task text, responses, and identifiers are synthetic
sanitization values. No private recording, command, hook output, encrypted blob,
path, or identifier is checked in.

Shapes were verified in local recordings on 2026-10-19:

| Fixture | Recorded source | Preserved structural evidence |
| --- | --- | --- |
| `world-state.jsonl` | Codex 0.163.0, 2026-10-19 | `world_state`, boolean `payload.full`, object `payload.state` |
| `compaction.jsonl` | Codex 0.163.0, 2026-10-17 | `response_item/compaction`, string ID and encrypted content, object metadata |
| `last-prompt.jsonl` | Claude Code 2.1.302, inspected 2026-10-19; record has no timestamp | `type`, `leafUuid`, `sessionId` |
| `session-hook.jsonl` | Claude Code 2.1.302, 2026-10-19 | `attachment/hook_success`, `hookEvent=SessionStart`, text fields, numeric duration, zero exit code |

All context-bearing text is replaced with `[redacted]`; IDs, timestamps, ordinal,
and timing values are fixed examples. World-state keys are reduced to a subset;
values and nested metadata are replaced. The compaction string is a placeholder,
not ciphertext. The hook command is a placeholder and is never executed. Parent
links in the last-prompt composite are explicitly constructed to describe one
linear retained turn; its leaf points to that turn's first message. This exercises
the safe case, not an assertion about an original session's branch history.

## Interpretation and limits

- **World state:** exactly `full` (boolean) and `state` (object) in the payload.
  Both full and delta-shaped records are acknowledged, but neither is applied or
  reconstructed. State contents remain opaque. A source-line coverage notice
  discloses that supplied context is unmodeled.
- **Compaction:** exactly `type`, nonempty string `id`, nonempty string
  `encrypted_content`, and object `internal_chat_message_metadata_passthrough`.
  Only occurrence is recorded; no decryption, content reconstruction, or inference
  about retained context is attempted.
- **Session-start hook:** only the observed `hook_success` / `SessionStart` shape
  with exactly its ten fields is supported. All eight text fields must be strings;
  hook name and tool-use ID must be nonempty. Exit code must be zero and duration
  nonnegative. The enclosing event needs a node ID and matching session ID; sidechain
  records are rejected. Other attachment variants are rejected. Output may have
  supplied context, so the report explicitly discloses that it is uninterpreted.
  Neither the command nor the output enters the run model or normal observations.
- **Last-prompt:** the observed record is a session-navigation pointer, not a user
  task or an acquisition. A real example pointed to a session-start attachment,
  so assuming it always means the latest user message would be unsafe. It is
  discarded only after validating matching session identity, a resolvable leaf,
  and one explicit linear sequence of message/hook node IDs and parent links.
  The first parent must be null; each subsequent parent must name the immediately
  previous node. Missing links/ancestors, duplicate IDs, branches, sidechains,
  extra bookkeeping fields, and unresolved leaves fail explicitly. The pointer
  never selects a branch or truncates the supplied run. General branch/session
  reconstruction remains unsupported.

Coverage notices are separate from acquisition/observation events and retain
physical source line numbers in input order. They contain fixed descriptions,
not sensitive values. Existing completion evidence remains authoritative; these
records cannot establish a task, tool observation, or successful execution.
Unknown fields in the recognized context payload shapes are rejected; arbitrary
contents *inside* the explicitly opaque state/metadata objects are not interpreted.
