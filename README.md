# unfriction

Inspect a recorded coding-agent task run: its task, requested repository context,
observations, attempted file modifications and their results, final response, and execution state. Supports reduced single-turn
Codex and Claude Code JSONL traces. Requires Go 1.26 or newer.

```sh
go build -buildvcs=false -o unfriction ./cmd/unfriction
./unfriction inspect fixtures/codex-acquisition.jsonl
./unfriction inspect fixtures/claude-acquisition.jsonl
```

Both examples begin with:

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

## Select a turn from a session

An explicit turn selector opts into session import. Use the recorded Codex turn
ID or Claude user-message UUID, prefixed with the family; display positions are
not selectors:

```sh
./unfriction inspect --format codex --turn codex:second -- fixtures/sessions/codex.jsonl
./unfriction inspect --format claude-code --turn claude-code:second -- fixtures/sessions/claude.jsonl
```

These synthetic examples select the second prompt, `Go`, and report only its
following activity. The report labels previous agent and earlier user context
separately, includes session/turn provenance, and retains physical source lines.
Session import never writes a reduced trace or executes recorded commands.

The same command without `--turn` keeps strict prepared-trace validation; a whole
multi-turn recording is not accepted implicitly. `--format` alone does not enable
session import. Unsupported selected tools, unresolved ancestry, sidechains,
conflicting message identities, and ambiguous records return diagnostics rather
than a fabricated report. A recording changed during inspection requires retry
or reselection. Optional usage measurements keep their existing scope/version
limits.

See [current session evidence](fixtures/sessions/current-formats.md) for the
sanitized current-format examples, verified envelope relationships, and remaining
limitations. Shell selection and automatic local discovery are separate planned
features; explicit turn selection does not require them.

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
It is not a general shell classifier. Without `--turn`, reduce recordings to one task first; see
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
./unfriction inspect fixtures/repeated-evidence/codex-0.163.0.jsonl
```

There is no fixed repetition count: both adapters preserve repeated requests and
feedback, including three or more acquisitions and actions to the same file.
Reads and actions need not alternate, and feedback may arrive after later requests.
Local IDs (`a1`, `a2`, ... and `act1`, `act2`, ...) identify each request independently
of its target. An error followed by another attempt remains recorded evidence;
the report does not infer why the agent continued or whether the task succeeded.
Both examples come from reduced real recordings. Structured Codex records combine
an operation and its feedback; each normalized pair shares a source ID and line.
The report discloses that event order follows the recorded items, without
reconstructing operation start order. Supported structured records use one
representation per turn; mixing them with legacy task/tool/final records fails
explicitly. See
[repeated-sequence provenance](fixtures/README.md#m4-repeated-sequence-evidence).

All events retain source order and correlate observations/results by request ID.
Missing observations are reported explicitly, including in completed runs.
`COMPLETED` requires explicit termination evidence; it does not establish either
complete trace capture or independently verified engineering success. Valid
unfinished traces report `INCOMPLETE` and exit zero. Malformed, contradictory,
unmatched, or unsupported events return a nonzero exit code with a diagnostic on
stderr and no report. Adapter validation errors include the source line where
applicable.

## Observable measurements

Acquisition-bearing reports append counts, normalized returned payload bytes,
literal target repetition, and attributable recorded operation durations.

| Measurement | Unit and boundary |
| --- | --- |
| [Frequency](docs/acquisition-measurements.md) | Requests, matched observations, and missing observations; attempts = matched + missing. |
| [Returned volume](docs/acquisition-volume.md) | UTF-8 text bytes and validated decoded image bytes separately; missing is not observed empty. |
| [Targets](docs/acquisition-targets.md) | Distinct literal target strings and additional requests; no path normalization or redundancy judgment. |
| [Timing evidence](docs/m5-timing-evidence.md) | Recorded seconds per acquisition where verified, otherwise unavailable; no total across operations. |
| [Model usage evidence](docs/m5-usage-evidence.md) | Verified vendor token counters at session-snapshot or model-message scope; never allocated to acquisitions. |

Model usage appears separately when telemetry is present. No section means no
supported usage was measured, not zero tokens. Unverified versions, malformed
counters, and conflicting snapshots remain unavailable. Codex cumulative session
snapshots may include earlier tasks; Claude message snapshots are deduplicated by
message identity. Counter categories remain vendor-specific and are not blindly
summed. These measurements do not establish usefulness, retained context, monetary
cost, or engineering success. Supplied context and opaque compaction are excluded.

Try the sanitized evidence:

```sh
./unfriction inspect fixtures/timing-evidence/codex-0.163.0.jsonl
./unfriction inspect fixtures/usage-evidence/codex-usage.jsonl
./unfriction inspect fixtures/usage-evidence/claude-usage.jsonl
```

Run checks:

```sh
go test -v ./...
go vet ./...
```

Acceptance tests build and invoke the real binary against sanitized recordings.
Focused unit tests cover parsing, association, ordering, rendering, and CLI format
selection. See [testing guidance](AGENTS.md) and
[contribution and naming conventions](CONTRIBUTING.md).

The Cobra CLI in `internal/cli` uses `internal/traceformat` for automatic selection,
or selects `internal/codex` or `internal/claude` explicitly.
Both adapters return the vendor-independent `internal/run` model, rendered by
`internal/report`. The event stream distinguishes context acquisition from supported file-modification
attempts. General action classification, automatic loop segmentation, comparisons,
and outcome verification remain out of scope.

See [release policy and Homebrew installation](docs/releases.md). Development
builds report `unfriction dev`; release builds embed their version. CI runs tests,
vet, release candidate builds, and Homebrew smoke checks for the M1, M2, M3,
repeated Claude and structured Codex sequences, observed tool-variant fixtures, and timing/usage evidence.

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
