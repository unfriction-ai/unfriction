## Contribution conventions

Follow [CONTRIBUTING.md](CONTRIBUTING.md) for milestone, issue, PR, and commit
naming conventions.

## Testing approach

Use outside-in TDD for behavior changes:
- Start with a failing acceptance test for the observable behavior.
- Use focused unit tests to drive parsing, reconstruction, and other detailed rules.
- Keep end-to-end tests focused on representative scenarios; cover edge cases
  primarily with unit tests.
- Test behavior and invariants, not plain data structures or implementation details.
- Keep existing acceptance scenarios passing.

## Implementation languages and repository tooling

- **Go is the default** for application code, tests, and reusable tooling checked
  into this repository. Prefer the existing Go toolchain rather than adding a
  second runtime for incidental automation.
- Keep one-off experiments, local analysis scripts, and utilities that require
  private history recordings **outside the source repository**. Commit only
  sanitized evidence and methodology that are useful to maintainers.
- Introduce another implementation language or runtime only when there is a
  concrete, documented benefit that justifies its maintenance and setup cost;
  explain that choice in the issue or PR and cover the tooling with appropriate
  tests and usage instructions.
- Small shell commands and scripts for build, release, and CI orchestration are
  acceptable; this convention is not a requirement to rewrite existing shell
  integration in Go.
- Do not turn a disposable experiment into permanent repository tooling by
  default. If the workflow becomes recurring, propose a maintained Go solution
  separately rather than preemptively porting the experiment.
