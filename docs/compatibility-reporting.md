# Public compatibility reports

An `inspect` detection or adapter-validation failure keeps its local diagnostic
and appends a plain HTTPS URL on its own line. It opens an editable new-issue form
in the public `unfriction-ai/unfriction-releases` repository. No request is sent by
the CLI, no issue is created automatically, and no token is needed by the CLI.
GitHub may require sign-in to review/submit the draft. Successful imports, bad CLI
arguments, file-open/read failures, and report-write failures do not offer vendor
compatibility reports.

## Metadata and privacy boundary

`internal/diagnostic` gives detector/adapter failures typed categories and source
positions, preserving their original error chains. `internal/compatreport` builds
the draft **without reading the diagnostic error string**. Public data consists of:

- Embedded binary version (the same value used by `--version`, sanitized).
- Selected/detected family, or unknown on failed detection. Explicit overrides
  are labeled as a selection with an unverified producer.
- Best-effort agent version from Codex `session_meta.payload.cli_version`, or
  Claude Code top-level `version` on structurally recognizable messages/hooks
  carrying session metadata. Session identifiers are never included.
- A fixed failure category, allowlisted record type/subtype, and bounded positive
  source-line number when available. Absent, empty, redacted, and unavailable
  record fields are distinguished. Unknown type strings are redacted, even if
  they look like ordinary identifiers. Fields unavailable for end-of-input or
  navigation validation may be reported as unavailable/unknown.
- A diagnostic assembled from those safe fields, plus editable placeholders for
  reproduction steps and an optional **manually sanitized** minimal trace.

Record type/subtype metadata describes top-level types and Codex payload or Claude
attachment subtypes. It never includes tool arguments, prompt or response text,
output, environment values, session IDs, local paths, encrypted context, or raw
JSON. The local stderr error remains unchanged and can contain private details;
review it before copying it into a public report. Public bodies do not link back
to private source issues or include automatic closing references.

Versions accept only `dev` or bounded numeric three-part versions, an optional
`v` prefix, and optional `alpha`, `beta`, or `rc` suffix with a numeric component.
Other strings, including arbitrary build metadata, become `unknown`. This
intentionally sacrifices some version precision rather than exposing unchecked
metadata. Conflicting version records also become unknown.

## Resource and URL limits

A reader wrapper captures at most **64 KiB** already consumed by detection/parsing;
it does not reopen the input or read ahead to obtain a version. On failure, version
discovery examines at most **64 nonblank records**, no later than the failing
source line when known. Metadata beyond either bound, after an early failure, or
in a record cut at the byte limit is unavailable. The complete input still reaches
the selected adapter unchanged.

The URL uses standard URL-encoded `title` and `body` query parameters. It is capped
at **2,048 bytes**. If a safe supported prefill cannot be produced, the fallback is
the ordinary public new-issue page. Neither route submits anything automatically.

## Validation

Executable acceptance tests exercise Codex, Claude Code, unknown format, embedded
release version, explicit override labeling, missing/late version metadata, and
success/file-access behavior. Unit tests cover privacy sanitization, URL decoding,
unknown/ambiguous/undetermined categories, metadata budgets, conflicting versions,
no extra reads, and ordinary-error preservation. Existing adapter acceptance tests
continue to assert their original diagnostic text and normalized reports.

A read-only smoke check on 2026-10-19 confirmed that the destination repository is
public with issues enabled. GitHub responded to an actual generated draft URL with
a sign-in redirect whose `return_to` preserves the complete title/body query. No
issue was submitted. Browser-control tooling is unavailable in this session, so
visual verification of the editable form while signed in remains outstanding.
