## Testing approach

Use outside-in TDD for behavior changes:
- Start with a failing acceptance test for the observable behavior.
- Use focused unit tests to drive parsing, reconstruction, and other detailed rules.
- Keep end-to-end tests focused on representative scenarios; cover edge cases
  primarily with unit tests.
- Test behavior and invariants, not plain data structures or implementation details.
- Keep existing acceptance scenarios passing.
