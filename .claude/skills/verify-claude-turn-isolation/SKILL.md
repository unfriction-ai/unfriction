---
name: verify-claude-turn-isolation
description: Exhaustively check that `unfriction inspect --turn claude-code:<uuid>` reports only the selected turn's records and returns a diagnostic for unsafe ancestry. Use when a change touches Claude Code session reconstruction (internal/claude/session.go readSession or ParseTurn, the session-mode path of internal/claude/parse.go, internal/session, or run.Acquire/RecordResult), when a test about parentUuid ancestry, sidechains, tool-result ownership, or earlier-turn context changes, or before merging any change to how Claude turns are selected.
---

# Verify Claude Code turn isolation

## What is verified

**Property (P1, turn isolation).** When `ParseTurn` returns a report for a
selected Claude Code prompt, the report's task is that prompt, its events are
exactly the tool calls and tool results whose recorded `parentUuid` chain
reaches that prompt before any other prompt, its final response is that turn's
own `end_turn` text, and its earlier-turn context (`EarlierUser`,
`PreviousAgent`) comes from the turn that the prompt's parent actually belongs
to. When the recording has a cycle or a dangling `parentUuid` anywhere, a
sidechain in the selected prompt's ancestry or in the turn, branched assistant
continuations in the turn, or a tool result whose request is missing, later in
the file, or owned by a different turn than the selected one, the outcome is a
diagnostic, never a report.

**Harm if broken.** A user reads a report that attributes another turn's file
reads, results, or final answer to the turn they selected, or shows the wrong
earlier context, with nothing indicating it is wrong.

**Guard (P2, not vacuous).** Every turn of linear multi-turn sessions built
from supported shapes (prompt/final, prompt/call/result/final, with system
nodes and repeated calls) is accepted and passes the P1 report check. A change
that makes P1 pass by rejecting everything fails P2.

**Property (P3, deterministic outcome).** The same recording bytes and selector
always give the same outcome: the same report, or the same diagnostic text.
Harm if broken: the same file produces different diagnostics, source lines, and
compatibility-report URLs from run to run.

## How it works

`internal/claude/turn_isolation_verify_test.go` (build tag `verify`, so plain
`go test ./...` and CI do not run it) enumerates **every** recording of 1..N
records over this alphabet, writes each as real Claude Code JSONL, and calls
the real `ParseTurn` for every prompt:

- record kinds: user prompt string, assistant `Read` `tool_use` (`stop_reason`
  `tool_use`), assistant text (`end_turn`), user `tool_result`, `system`
  `turn_duration` node;
- `parentUuid`: `null`, a uuid that no record has, or any record (itself and
  later records included, so cycles and forward edges are generated);
- each `tool_result` references any recorded call (earlier or later in the
  file) or a tool id that no record has;
- every recording once with no sidechain and once per record as the single
  `isSidechain` record.

An independent oracle in the test computes ownership by walking `parentUuid`
on the abstract graph (never calling adapter code) and decides what must be
rejected and what an accepted report must contain. A diagnostic is always an
allowed outcome for P1; only accepted reports are checked against the oracle.

Bounds: P1 at N=4 records (6,566,806 recordings, about 76 s on a 10-core
Apple Silicon laptop; 4 is the smallest bound that holds a complete
prompt/call/result/final turn). P3 at N=3 (80,326 recordings, six parses per
selector, about 3 s when failing on F1 and about 11 s when passing). P2 is
27 fixed sessions of up to 21 records.

Assumptions: one session id (`"s"`), unique uuids, no `message.id`,
`requestId`, or `apiBlockIndex` grouping, no attachments or `last-prompt`
navigation records, `Read` with text results only.

## When to run it

Run it when your diff touches any covered file in the coverage record below.
To see whether the covered code changed since the verification was last
confirmed:

```sh
git diff --stat cbc2c3ce0dabc84f6db5d42858f88749cabb6534..HEAD -- internal/claude/session.go internal/claude/parse.go internal/session/session.go internal/run/run.go internal/source/records.go
```

## Run it

From the repository root (Go version from `go.mod`, 1.26 or newer; confirmed
with go1.27.3):

```sh
go test -tags verify ./internal/claude -run Verify -count=1 -v
```

Fast loop while iterating (P1 at 3 records, about 1 s; it already catches all
five scratch breaks listed below):

```sh
UNFRICTION_VERIFY_RECORDS=3 go test -tags verify ./internal/claude -run 'VerifyTurnIsolation|VerifyLinear' -count=1 -v
```

`UNFRICTION_VERIFY_RECORDS` overrides the bound for every test. Always finish
with the default bounds before handing off. Also run `go vet -tags verify
./internal/claude`.

## Expected baseline at the sync commit

- `TestVerifyTurnIsolation`: PASS, logging `checked 6566806 recordings of up
  to 4 records; 59661 selected turns produced a report`.
- `TestVerifyLinearSessionsAccepted`: PASS.
- `TestVerifyDeterministicOutcome`: **FAIL on known finding F1 only.** The last
  log line reads `... match known finding F1 ..., 0 are NEW`. Any `NEW` count
  above 0, or any `[NEW]` counterexample, is a new P3 violation.

### Known finding F1 (open)

`readSession` validates parent edges with `for id := range g.nodes` in
`internal/claude/session.go`. Go randomizes map iteration, so when a recording
has more than one bad node, the node reported (its line, its record type, and
whether the message is `cycle in recorded ancestry` or `missing required
ancestor`) changes between runs. Smallest counterexample in user terms:

1. Line 1: user prompt `n0` with `parentUuid` `"gone"` (no such record).
2. Line 2: system `turn_duration` node `n1` with `parentUuid` `"n1"`.
3. Run `unfriction inspect --format claude-code --turn claude-code:n0 -- f.jsonl`
   30 times: 28 runs print `line 1: missing required ancestor` with a
   compatibility URL for a `user` record at line 1; 2 runs print `line 2:
   cycle in recorded ancestry`, with a different URL.

No report is fabricated; the harm is unstable diagnostics. A scratch fix that
iterates the records in file order makes P3 pass with P1 and P2 still passing.
When F1 is fixed, delete the F1 sentence from this section, update the
baseline to PASS, and keep `ancestryDiagnostic` only if it still helps triage.

## Reading a failure

Each counterexample prints the property it broke, the records line by line
(`line 3: n2 user tool_result for t1, parentUuid null`), and the exact JSONL.
The three smallest are shown.

1. Paste the printed JSONL into a file and reproduce with the CLI:
   `go run ./cmd/unfriction inspect --format claude-code --turn claude-code:<prompt uuid> -- <file>`.
2. Decide which side is wrong:
   - **Real bug**: the CLI prints a report and the counterexample is a
     recording the README promises to reject (unresolved ancestry, sidechain,
     cross-turn result, branched/ambiguous assistant), or the report lists an
     event, final response, or earlier context the turn does not own by
     `parentUuid`. Fix product code; do not edit the oracle to match.
   - **Modeling error**: the oracle demands something the product never
     promised (for example the change intentionally accepts a new, documented
     shape such as parallel assistant blocks sharing a `message.id`). Update
     the oracle in `mustReject` or `checkReport` together with the README or
     fixture docs that state the new rule, and say so in the PR.
   - **Generator error**: the JSONL itself is not what the record description
     says. Fix `vGraph.json`.
3. Never lower the default bound, drop alphabet entries, or skip a property to
   make a run pass.

## Updating the verification for an intended change

- New supported record kind or field in session reconstruction (for example a
  new ancestry node type in `knownSessionRecord`): add a kind constant, its
  JSONL in `vGraph.json`, its description in `describe`, and its ownership
  and report rules in the oracle. Keep the P1 default bound at 4 unless the run
  exceeds about two minutes; if so, record the new bound and timing here.
- New rejection rule: add the condition to `mustReject` only if the product
  promises it; extra rejections are always allowed by P1.
- New report content derived from ancestry: extend `checkReport`.
- After the change, rerun the full command, update the baseline numbers above,
  then update the coverage record.

## Coverage record

Last confirmed against commit `cbc2c3ce0dabc84f6db5d42858f88749cabb6534`
(branch `m6`, 2026-10-29).

| Check | Covers |
| --- | --- |
| P1 TestVerifyTurnIsolation | `internal/claude/session.go`: `readSession` (session identity, node identity/parent decoding, prompt turns, parent-edge validation, ownership, assistant-continuation check, tool request/result ownership, `End`/`Final`/`Completed`, `PreviousTurnID`/`PreviousAgent`/`EarlierUser`), `ParseTurn` (sidechain ancestry, owned-record selection, coverage split); `internal/claude/parse.go`: `parse` with `sessionMode` true for prompt, `Read` `tool_use`, text `tool_result`, `end_turn`; `internal/session/session.go`: `Document.Find`; `internal/run/run.go`: `Acquire`, `RecordResult`/`Observe`; `internal/source/records.go`: `ReadAll`, `Records` |
| P2 TestVerifyLinearSessionsAccepted | same functions, acceptance side |
| P3 TestVerifyDeterministicOutcome | `readSession` and `ParseTurn` end to end, including diagnostics |

Move the sync commit forward only after you have read the diff of the covered
files since the recorded commit, confirmed that the generator and oracle still
describe that code, and seen the expected baseline at the new commit. Update
the commit, the date, and the baseline numbers together.

## Not covered

- Recordings larger than 4 records (P1) or 3 records (P3); a pass is a proof
  only up to those bounds.
- `message.id`/`requestId`/`apiBlockIndex` block grouping and its exemption in
  the continuation check, duplicate uuids, conflicting session ids,
  attachments, `last-prompt` navigation, `Edit` actions, image results, usage.
- Codex session reconstruction (`internal/codex/session.go`), format
  detection, and the recording-changed-during-inspection check in
  `internal/inspection/inspect.go`.
- Which turn the `select` picker offers; only `ParseTurn` given a uuid is
  checked.

## Proof that the check can fail

Each scratch break below was applied to `internal/claude/session.go`, caught
by `TestVerifyTurnIsolation` at 3 records, and reverted:

1. Disable the `ambiguous assistant continuations` check: caught as branched
   assistants accepted (prompt with two assistant children).
2. Drop the owner comparison from the tool-result check: caught as a tool
   result crossing the selected turn's ownership.
3. Select records by file position (`turn.Start`..`turn.End`) instead of
   ownership: caught as another turn's final response in the report.
4. Take earlier context from the previous turn in file order instead of the
   parent's owner: caught as wrong `EarlierUser`.
5. Disable the sidechain-ancestor check in `ParseTurn`: caught as a sidechain
   ancestor accepted.
