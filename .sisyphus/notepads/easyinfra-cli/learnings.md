
## T8 (config loader/validator)
- `errors.Join` works well for accumulating multi-validation errors; preserves all messages.
- DFS cycle detection: track `visiting`/`visited` states + path stack to reconstruct cycle.
- `FakeRunner.Default` provides catchall response for tests where exact arg matching isn't required.
- Test fixtures live at `testdata/infra/` relative to package dir (Go tests run from package dir).
- Achieved 98.3% coverage on `pkg/config/` (target ≥85%).

## T12: backup package
- pkg/k8s.Client is a concrete struct (no interface) — for tests inject a FakeRunner into k8s.Client{Runner: fake} and reuse for backup.Manager.
- FakeRunner key format: `name + " " + strings.Join(args, " ")` — for dynamic args (timestamps), use a wrapper Runner that inspects the joined command string.
- Backup orchestration: save replicas → ListDeployments → scale 0 → WaitForPodsDeleted → ssh tar (per pvc) → scale back up (even on tar failure) → scp -r → ssh rm.
- Always restore replicas in defer-like fashion (errors.Join collects errors) so a failing tar still scales pods back up.
