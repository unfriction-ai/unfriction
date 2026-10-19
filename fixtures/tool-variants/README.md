# Observed tool variants

These fixtures were prepared on 2026-10-19 from the real private-history slices
that the initial M3 implementation rejected. They are sanitized reductions, not
new recordings made with the displayed tasks. Private originals and extraction
scripts remain outside the repository.

| Fixture | Recorded version | Selected task span (UTC) | Original source lines retained |
| --- | --- | --- | --- |
| `codex-create` | Codex `0.138.0-alpha.7` | 2026-06-11 18:42:52–18:48:12 | 57, 61, 96, 97, 109, 111, 226, 229 |
| `codex-delete-lines` | Codex `0.138.0-alpha.7` | 2026-06-11 18:58:13–19:02:06 | 304, 308, 361, 363, 390, 393 |
| `codex-rejected` | Codex `0.140.0-alpha.2` | 2026-06-13 16:12:12–16:23:13 | 293, 296, 423, 425, 481, 484 |
| `claude-image-png` | Claude Code `2.1.294` | 2026-10-12 08:31:56–11:43:17 | 204, 216, 217, 246 |
| `claude-image-jpeg` | Claude Code `2.1.229` | 2026-08-14 08:06:10–08:06:42 | 558, 561, 562, 569 |
| `claude-offset-edit` | Claude Code `2.1.278` | 2026-09-22 16:41:16–16:43:41 | 3, 47, 48, 53, 54, 135 |

Each `.jsonl` has a corresponding expected `.txt` report. Codex slices retain the
recorded lifecycle markers and selected call/result pairs. Claude slices retain
the task, selected tool-use/result blocks, and final text with `end_turn`, after
following the recorded parent chain. Other tools, intermediate text, context and
session records, and optional metadata were removed. The slices are not complete
inventories of their original tasks; final responses can depend on omitted work.

Sanitization replaces tasks, final responses, paths, IDs, patch/edit contents,
read text, and successful patch output with fixed example values. Codex completion
text is replaced consistently. Timestamps are omitted. The creation fixture's
read originally targeted a context/skill document, which is represented by
`GUIDE.md`; it is not claimed to be a read of the newly created file. The deletion
fixture retains the observed two-hunk structure: a removal-only hunk followed by
a replacement hunk. Patch lengths and contents are reduced. Event shapes, field
value types, ordering, associations, and explicit termination markers are retained.

The image fixtures replace private screenshot/photo bytes with generated one-pixel
PNG and JPEG images. They retain the observed one-element content array with
`type=image` and `source.type=base64`, `source.media_type`, and `source.data`.
Original dimensions, image bytes, and visual content are not retained. The report
shows only media type and decoded byte count, explicitly without interpretation.

The standalone Codex result `patch rejected by user` is preserved verbatim. It is
a refusal, not evidence that the patch executed or that the task failed. It does
not set the tool-execution error flag; its text explicitly states the rejection.

The offset-shaped Claude call used string fields `file_path`, `offset`, and
`new_text`. The fixture substitutes offset `"0"` for `"1884"`. Its exact generic
error text and `is_error=true` are retained: Edit was disabled in that session.
This is evidence of an attempted modification with a rejected tool call, **not**
proof of a valid/successful Claude offset-edit API. No offset units, extent, or
resulting file contents are inferred.

## Supported limits

- Codex: one `Add File` with at least one `+` body line, or one `Update File`
  with bare `@@` hunks. Every update hunk must include a removal or addition;
  removal-only and insertion-only hunks are accepted. No whole-file deletion,
  rename, multiple files, named anchors, or other patch grammar is inferred.
- Codex results: the existing exit-code envelope or the exact observed standalone
  refusal. Unknown plain-text variants remain unsupported.
- Claude: the original replacement Edit shape, or exactly `file_path`, a
  nonnegative decimal string `offset`, and nonempty string `new_text`. Inputs
  mixing those schemas, unknown fields, nulls, and numeric offsets are rejected.
- Claude image observations: exactly one inline base64 PNG or JPEG image returned
  for a read. Base64 encoding and image headers are checked against the declared
  media type; pixels are not interpreted and bytes are not retained in the domain
  model. Mixed/multiple blocks, remote image URLs, unsupported media types, and
  malformed encodings fail explicitly. An image is never fabricated as text.
- Images use the same request/result association, duplicate-result checks, source
  ordering, and missing-result rules as text observations. Tool errors, refusals,
  image receipt, and file creation attempts do not establish execution completion
  or independently verified task correctness.

Acceptance tests run every fixture with automatic detection and explicit format
selection, compare complete reports, and verify that inspection leaves target
files unchanged and never creates the requested file. Focused tests cover invalid
variants, association, refusal without completion, and malformed image results.

## Local compatibility replay

The audit first extracted 21,492 overlapping candidate slices from 6,217 local
history files. The original implementation imported 21,456 and rejected 36:
21 image reads, 10 file creations, 3 updates with removal-only hunks, 1 standalone
user refusal, and 1 offset-shaped Edit attempt. Replaying the exact same inputs
after this change imports all 21,492; all 21,456 previously accepted reports are
byte-for-byte unchanged. Existing missing-result diagnostics remain intact.

These numbers measure a selected corpus of reduced traces, not full-history
compatibility, unique tasks, or engineering correctness. The original private
traces, generated extracts, per-file audit reports, and disposable scripts are
not checked in. Only the sanitized regression evidence and methodology live here.
