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

## How the comparison was performed

The 2026-10-19 comparison used a **private**, locally selected manifest of twenty
Codex and twenty Claude Code histories. It snapshotted each input once, ran both
the baseline and candidate unfriction binaries against identical snapshots with
automatic format detection, and summarized the **first error category and line**
or successful import for each run. Runs were time-limited; only aggregated
categories, line numbers, and a corpus digest were committed.

The helper used for this one-off experiment was deliberately removed from the
repository. No maintained batch-comparison command is currently provided. To
repeat this method, keep the manifest and trace snapshots outside the source
repository, compare the same set of inputs against both binaries, sanitize the
results, and preserve a digest so different runs can be distinguished. Never
commit raw traces, private paths, prompts, commands, or error bodies.

**Reproducibility limitation:** the original forty-history manifest was not
retained in this repository. The recorded aggregates and corpus digest remain
useful historical evidence but are not enough to reconstruct the private corpus
or prove that it matches the original experiment. Repeated evaluation tooling
should be proposed separately if it becomes a recurring need.
