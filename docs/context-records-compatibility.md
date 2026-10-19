# Context-record compatibility comparison

Issue #10 is a post-M2 compatibility increment, not a full-history importer.

## Evidence and corpus limitation

The issue reports 20 Codex and 20 Claude Code full histories, but does not attach
the original manifest or selection command. That selection has been requested.
**Exact identity with the original forty histories is not yet verified.**

As a separate reproducible check on 2026-10-19, selected the twenty most recently
modified JSONL recordings under each local agent's history directory (descending
mtime, path as tie-breaker). This sample reproduces the issue's published baseline
counts and first-error locations, but matching aggregate results does not prove
corpus identity. Each selected history was snapshotted once; both binaries read
the identical snapshot without reduction. Paths and raw diagnostics remain private.
The aggregate [machine-readable report](context-records-compatibility.json) includes
a digest of the sampled contents and family labels, in manifest order.

Before: the merged auto-detection implementation (main `25cbbdd`, same tree as
`e17ae34`). After: the context-record changes in this worktree. Detection and
acquisition support are unchanged.

## First-failure distribution

| Family | Before | After | Histories |
| --- | --- | --- | ---: |
| Codex | `world_state`, line 7 | `thread_settings_applied`, line 9 | 9 |
| Codex | `world_state`, line 7 | `item_completed`, line 10 | 8 |
| Codex | Detection: unknown | Detection: unknown | 1 |
| Codex | Detection: prefix exhausted | Detection: prefix exhausted | 2 |
| Claude Code | `last-prompt`, line 1 | `mode`, line 2 | 11 |
| Claude Code | `attachment`, line 3 | Unsupported user text block, line 5 | 4 |
| Claude Code | `attachment`, line 3 | Unsupported attachment shape, line 5 | 4 |
| Claude Code | Detection: unknown | Detection: unknown | 1 |

Both families still import **0/20 complete histories**: Codex has 17 adapter and
3 detection failures; Claude Code has 19 adapter and 1 detection failure.
Recognized context records no longer conceal the next unsupported variant.
The four controlled single-turn regression fixtures pass, including the explicit
coverage disclosures. This result does not justify making unrelated records or
multi-turn histories permissive. Compaction is covered by its CLI regression
fixture; it was not a first-failure category in the automatically detected sample.

## Reproduction

Keep a private JSON manifest outside this repository:

```json
[
  {"family": "codex", "path": "/private/path/to/history.jsonl"},
  {"family": "claude-code", "path": "/private/path/to/another-history.jsonl"}
]
```

Provide exactly twenty distinct histories for each family, then run:

```sh
python3 scripts/compatibility.py \
  --manifest /private/path/to/forty-histories.json \
  --before /private/path/to/baseline-binary \
  --after /private/path/to/candidate-binary \
  --output /private/path/to/aggregate-results.json
```

The script snapshots each input for the two invocations, preserves automatic
format selection, imposes a per-invocation timeout, and records only allowlisted
aggregate diagnostic categories and source line numbers. It never writes source
contents, source paths, commands, hook output, or raw errors to the report.
Once the original manifest is supplied, rerun it to close the original-corpus
verification gap rather than presenting this secondary sample as identical.
