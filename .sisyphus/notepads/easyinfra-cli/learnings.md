
## T8 (config loader/validator)
- `errors.Join` works well for accumulating multi-validation errors; preserves all messages.
- DFS cycle detection: track `visiting`/`visited` states + path stack to reconstruct cycle.
- `FakeRunner.Default` provides catchall response for tests where exact arg matching isn't required.
- Test fixtures live at `testdata/infra/` relative to package dir (Go tests run from package dir).
- Achieved 98.3% coverage on `pkg/config/` (target ≥85%).
