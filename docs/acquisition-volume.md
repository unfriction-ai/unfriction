# Observed acquisition volume

For runs with acquisitions, `unfriction inspect` adds two independent totals to
Acquisition measurements:

- **Observed text:** normalized text observation bytes, measured with Go
  `len(observation.Text)`. UTF-8 multibyte characters and recorded newlines count
  by their byte lengths. Vendor result wrappers are already removed by the
  existing adapters; report formatting is not counted.
- **Observed images:** the sum of validated decoded image byte lengths retained
  in `ImageObservation.ByteLength`. Measurement does not decode images again,
  retain pixel contents, or interpret their meaning.

These totals describe **normalized observed result payload**, not original file
size, model input tokens, or effective context retained after compaction. They do
not imply that the payload was useful or that the task succeeded. No token or
monetary conversion is made.

The calculation reuses acquisition/observation associations from #26. Action
requests and action results are excluded. Tool-error observation text contributes
received bytes. An observed empty text result contributes zero; a missing
observation contributes no known bytes and remains separately disclosed by the
existing missing count and diagnostic. The two cases are not equivalent.

Acceptance tests exercise supported sanitized Codex and Claude Code records with
explicitly synthetic mutations for Unicode, empty, pending and error cases. The
Claude image fixture supplies validated 73-byte PNG evidence. Existing no-tool
output is unchanged.
