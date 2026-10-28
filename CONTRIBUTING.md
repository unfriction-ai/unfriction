# Contributing

Use these conventions when creating milestones, issues, pull requests, and
commits in this repository.

## Milestones and issue assignment

Milestones describe delivery outcomes. Name them `M<number> — <delivery outcome>`,
for example `M4 — Repeated Acquisition–Action Feedback`. Put the intended outcome
and completion criteria in the milestone description.

Issues describe work to be done. Assign an issue to a product milestone through
GitHub metadata when it contributes to that outcome. Independent maintenance and
documentation issues need no product milestone. Keep milestone numbers out of
issue and PR titles and do not use milestone identifiers as scopes.

Pull requests describe the final delivered change. Commits record the changes
merged into the repository; their messages are inputs to release automation.

## Issue, PR, and commit naming

Use the Conventional Commit title format `type(scope): description` for issues,
PRs, and squash commits. Use Conventional Commits for commits merged to `main`.
Keep PR and squash-commit titles aligned with the final delivered change,
updating them when the scope changes.

Start the description with a lowercase action verb and state the concrete
behavior or outcome concisely. Avoid roadmap labels and vague descriptions such
as “updates” or “improvements.”

Choose the type that describes the work:

| Type | Use for |
| --- | --- |
| `feat` | New capabilities |
| `fix` | Corrections |
| `docs` | Documentation |
| `chore` | Maintenance |
| `test` | Test-only changes |
| `refactor` | Restructuring without a behavior change |

Choose the scope by the area affected. Reuse established areas such as `inspect`,
`codex`, `adapters`, `cli`, `release`, `distribution`, and `tooling`. These examples
are guidance, not an exhaustive registry.

Examples from the repository's normalized issues:

```text
feat(inspect): reconstruct repeated acquisition-action feedback within a run
fix(codex): support structured item_completed records for repeated feedback
docs(distribution): document Homebrew installation and public releases
chore(tooling): remove one-off compatibility tooling and document Go-first conventions
```

## Releases

Merged commit messages drive semantic releases. Before merging, check that the
commit message, including the squash title when squashing, accurately describes
the delivered change. Renaming an issue does not trigger a release; issue titles
and milestone assignments are planning metadata.

See the [release policy](docs/releases.md#semantic-version-policy) for version-bump
and breaking-change rules.
