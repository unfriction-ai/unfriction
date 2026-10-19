# unfriction

Inspect a recorded coding-agent task run: its task, requested repository context,
observations, final response, and execution state. Supports reduced single-turn
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

M2 supports the observed Codex `exec_command` form `cat <one-literal-path>` and
Claude Code `Read` with text results. It is not a general shell classifier or a
full-session importer. Reduce recordings to one task first; see
[fixture provenance and input contracts](fixtures/README.md) for the precise
supported variants and sanitization limitations.

Recognized environment state, opaque compaction, and session-start hooks produce
explicit context-coverage notices without exposing their contents or attributing
them to the agent. Navigation bookkeeping is accepted only for a verified linear
turn. See [record-specific limits](fixtures/context-records/README.md).

Context events retain source order and correlate observations by request ID.
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
`internal/report`. The acquisition phase sits between the task and final response;
no action classification, repeated acting loops, metrics, comparisons, or outcome
verification are included.

See [release policy and Homebrew installation](docs/releases.md). Development
builds report `unfriction dev`; release builds embed their version. CI runs tests,
vet, release candidate builds, and Homebrew smoke checks for all three fixtures.
