# unfriction

Inspect a recorded coding-agent task run: its task, requested repository context,
observations, final response, and execution state. Supports reduced single-turn
Codex and Claude Code JSONL traces. Requires Go 1.26 or newer.

```sh
go build -buildvcs=false -o unfriction ./cmd/unfriction
./unfriction inspect fixtures/codex-acquisition.jsonl
./unfriction inspect --format claude-code fixtures/claude-acquisition.jsonl
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

The default format is `codex`. The existing no-tool path remains available:
`unfriction inspect fixtures/first-run.jsonl`.

M2 supports the observed Codex `exec_command` form `cat <one-literal-path>` and
Claude Code `Read` with text results. It is not a general shell classifier or a
full-session importer. Reduce recordings to one task first; see
[fixture provenance and input contracts](fixtures/README.md) for the precise
supported variants and sanitization limitations.

Context events retain source order and correlate observations by request ID.
Missing observations are reported explicitly, including in completed runs.
`COMPLETED` requires explicit termination evidence; it does not establish either
complete trace capture or independently verified engineering success. Valid
unfinished traces report `INCOMPLETE` and exit zero. Malformed, contradictory,
unmatched, or unsupported events return a nonzero exit code with a line-numbered
diagnostic on stderr and no report.

Run checks:

```sh
go test -v ./...
go vet ./...
```

Acceptance tests build and invoke the real binary against sanitized recordings.
Focused unit tests cover parsing, association, ordering, rendering, and CLI format
selection. See [testing guidance](AGENTS.md).

The Cobra CLI in `internal/cli` selects `internal/codex` or `internal/claude`.
Both adapters return the vendor-independent `internal/run` model, rendered by
`internal/report`. The acquisition phase sits between the task and final response;
no action classification, repeated acting loops, metrics, comparisons, or outcome
verification are included.

See [release policy and Homebrew installation](docs/releases.md). Development
builds report `unfriction dev`; release builds embed their version. CI runs tests,
vet, release candidate builds, and Homebrew smoke checks for all three fixtures.
