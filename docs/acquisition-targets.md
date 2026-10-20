# Literal acquisition targets

The acquisition measurements include distinct targets and repeated requests.
Targets are compared as recorded strings: `src/file.go` and `./src/file.go` are
two distinct targets. No trimming, case folding, path cleaning, filesystem access,
symlink resolution, or content comparison occurs.

Repeated requests equal the existing acquisition attempt count minus distinct
targets. Each later request of the same literal target contributes one repeat,
including requests with missing or error observations. Actions do not contribute
targets or requests, even when they change a file between reads. Every request
and observation remains a separate event with its original association.

For Read A, Act A, Read A, Read B, the result is three attempts, two distinct
targets, and one repeated request. This describes recorded exploration; it does
not establish redundancy, wasted work, usefulness, or task success. Reports with
no acquisitions retain their existing output.

CLI acceptance tests exercise both supported Codex and Claude Code formats using
explicitly synthetic mutations of existing sanitized action fixtures. They cover
interleaving, literal spellings, missing results, errors, and multiple actions.
Unit tests additionally enforce the count identity and preservation of events.
