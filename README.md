# unfriction

Inspect a recorded coding-agent task run. M1 accepts a single Codex turn
without tool calls and reports its task, final response, and execution state.

Requires Go 1.26 or newer.

Release automation and public Homebrew distribution are being implemented in
issue #3. See [release policy, setup, and installation](docs/releases.md).
Development builds report `unfriction dev`; release builds embed their version.

```sh
go build -buildvcs=false -o unfriction ./cmd/unfriction
./unfriction inspect fixtures/first-run.jsonl
```

```text
Task:
Reply with the word ready.

Final response:
ready

Execution state: COMPLETED
Completion describes execution termination, not verified task success.
```

`COMPLETED` requires an explicit completion event matching the turn and final
response. A trace with a task but no completion event is `INCOMPLETE`.
Malformed input, conflicting completion records, multiple turns, and tool calls
return a nonzero exit code with a diagnostic on stderr.

Run the acceptance scenario and checks:

```sh
go test -v ./...
go vet ./...
```

The single acceptance test file builds a fresh binary in a temporary directory,
invokes it against the real sanitized fixture twice, and checks the exact output.
It also exercises incomplete and invalid traces through that binary. VCS stamping
is disabled for the disposable binary so tests work independently of Git metadata.
GitHub Actions runs these checks on pushes and pull requests.

The Cobra command in `internal/cli` connects the Codex input boundary
(`internal/codex`) to a vendor-independent model (`internal/run`) and text report
(`internal/report`). No database, web UI, token analysis, or acquisition
classification is included. See [fixture provenance and format assumptions](fixtures/README.md).

M2 follow-up: import equivalent real, sanitized Codex and Claude Code runs that
acquire context and complete. Assert that both produce the same normalized task,
execution state, and final response through executable acceptance scenarios.
The Claude adapter and acquisition handling belong to M2.
