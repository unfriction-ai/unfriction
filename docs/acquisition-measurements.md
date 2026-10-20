# Acquisition measurements

`unfriction inspect` appends acquisition measurements when the reconstructed run
contains at least one acquisition request. Runs without acquisitions retain their
existing report unchanged.

- **Attempts** counts every acquisition request, including pending requests and
  repeated reads of the same target.
- **Matched observations** counts acquisition observations associated with those
  requests. Empty results, image results, and tool-reported errors count as matched.
- **Missing observations** counts requests without an associated observation,
  matching the existing missing-observation diagnostics.

For valid supported traces, `attempts = matched observations + missing observations`.
All counts are nonnegative integers. File modification/creation requests and their
results do not contribute. Counts use the reconstructed request/observation IDs,
not target equality, payload length, timing, or model usage. An observation or
completed execution is not evidence of verified task success.

The public CLI acceptance test covers a pending Codex read, Claude Code image and
error observations, an empty observation, interleaved reads/actions, and the
unchanged no-tool report. Its additional edge sequences are explicitly synthetic
mutations of supported sanitized fixtures, not new vendor telemetry claims.
