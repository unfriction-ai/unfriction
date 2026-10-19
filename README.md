# unfriction

Inspect a recorded coding-agent task run: its task, requested repository context,
observations, attempted file modifications and their results, final response, and execution state. Supports reduced single-turn
Codex and Claude Code JSONL traces. Requires Go 1.26 or newer.

```sh
go build -buildvcs=false -o unfriction ./cmd/unfriction
./unfriction inspect fixtures/codex-acquisition.jsonl
./unfriction inspect fixtures/claude-acquisition.jsonl
```

Both examples produce:

```text
Task:
Read README.md and report the project name.

Context events:
1. Acquisition a1: README.md
2. Observation for a1:
# Example

Final response:
The project is Example.

Execution state: COMPLETED
Completion describes execution termination, not verified task success.
```

Trace-family detection is automatic by default (`--format auto`). The detector
reads at most 64 KiB and inspects at most 64 nonblank complete JSONL records for
unambiguous structural evidence. It checks the whole bounded prefix for conflicts,
then passes the entire unchanged trace to the selected adapter. Filenames and
prompt text do not influence selection.

Use `--format codex` or `--format claude-code` to bypass detection for reduced
traces or troubleshooting. Unknown, ambiguous, or insufficient evidence produces
a nonzero error suggesting these overrides. Read errors also fail explicitly.
Recognizing a family does not validate its variant: the selected adapter validates
the full trace and never falls back to another adapter on failure.

The existing no-tool path remains available:
`unfriction inspect fixtures/first-run.jsonl`.

The acquisition path supports the observed Codex `exec_command` form
`cat <one-literal-path>` and Claude Code `Read` with text results. M3 additionally supports the recorded
`sed -n '1,220p' <path>` read, single-file Codex `apply_patch` creation/updates
(including deletion-only hunks), and Claude Code `Edit` replacement or recorded offset-shaped attempts. Claude image
reads return explicit PNG/JPEG observation metadata; image content is not
interpreted. Recorded user rejection remains visible as a result. See the
[observed tool variants](fixtures/tool-variants/README.md) for exact limits.
It is not a general shell classifier or a full-session importer. Reduce recordings to one task first; see
[fixture provenance and input contracts](fixtures/README.md) for the precise
supported variants and sanitization limitations.

Recognized environment state, opaque compaction, and session-start hooks produce
explicit context-coverage notices without exposing their contents or attributing
them to the agent. Navigation bookkeeping is accepted only for a verified linear
turn. See [record-specific limits](fixtures/context-records/README.md).

Inspect an acquisition followed by an attempted modification:

```sh
./unfriction inspect fixtures/codex-action.jsonl
./unfriction inspect fixtures/claude-action.jsonl
```

These reports show acquisition, observation, action target, and corresponding
result under `Run events:`. Actions without a prior acquisition are supported.
Inspection never executes recorded commands or applies changes. Tool errors are
reported as observed evidence, not verified task outcomes. Missing action results
remain explicit even when execution completed.

Inspect repeated acquisition and modification within one task:

```sh
./unfriction inspect fixtures/claude-repeated.jsonl
```

There is no fixed repetition count: both adapters preserve repeated requests and
feedback, including three or more acquisitions and actions to the same file.
Reads and actions need not alternate, and feedback may arrive after later requests.
Local IDs (`a1`, `a2`, ... and `act1`, `act2`, ...) identify each request independently
of its target. An error followed by another attempt remains recorded evidence;
the report does not infer why the agent continued or whether the task succeeded.
The Claude example comes from a reduced real recording. Codex repetition is
currently covered by synthetic regression sequences; two real repeated Codex
extracts have been located but use a structured record format the adapter does
not yet support. See
[repeated-sequence provenance](fixtures/README.md#m4-repeated-sequence-evidence).

All events retain source order and correlate observations/results by request ID.
Missing observations are reported explicitly, including in completed runs.
`COMPLETED` requires explicit termination evidence; it does not establish either
complete trace capture or independently verified engineering success. Valid
unfinished traces report `INCOMPLETE` and exit zero. Malformed, contradictory,
unmatched, or unsupported events return a nonzero exit code with a diagnostic on
stderr and no report. Adapter validation errors include the source line where
applicable.

Run checks:

```sh
go test -v ./...
go vet ./...
```

Acceptance tests build and invoke the real binary against sanitized recordings.
Focused unit tests cover parsing, association, ordering, rendering, and CLI format
selection. See [testing guidance](AGENTS.md).

The Cobra CLI in `internal/cli` uses `internal/traceformat` for automatic selection,
or selects `internal/codex` or `internal/claude` explicitly.
Both adapters return the vendor-independent `internal/run` model, rendered by
`internal/report`. The event stream distinguishes context acquisition from supported file-modification
attempts. General action classification, automatic loop segmentation, metrics, comparisons,
and outcome verification remain out of scope.

See [release policy and Homebrew installation](docs/releases.md). Development
builds report `unfriction dev`; release builds embed their version. CI runs tests,
vet, release candidate builds, and Homebrew smoke checks for the M1, M2, M3,
repeated Claude sequence, and observed tool-variant fixtures.

When detection or adapter validation fails, the CLI appends a link to an editable
[public compatibility issue draft](https://github.com/unfriction-ai/unfriction-releases/issues/new).
The URL includes the running binary's version, selected agent and best-effort
version, failure category, and allowlisted record names/source line. Unknown or
unsafe values are marked explicitly. An explicit `--format` identifies your
selected adapter, not proof of the trace's producer.

The CLI makes no network request, requires no GitHub token, and submits nothing.
Review the draft before submitting; GitHub may require sign-in. Prompt text, tool
output, paths, session IDs, and raw error text are never copied into the URL.
If you attach a minimal trace yourself, redact it first. Existing diagnostic
detail remains local on stderr. See [reporting limits](docs/compatibility-reporting.md).
