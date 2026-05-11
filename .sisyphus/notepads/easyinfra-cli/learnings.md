
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

## T15: init/update commands
- Inject manager-builder via package-var `newRepoManager` for testability without changing pkg/repo or pkg/paths.
- `paths.RepoDir()` returns `(string, error)` — callers must handle error path.
- Test helper swaps the var and restores via `t.Cleanup`. Use `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` to cover the default constructor.
- FakeRunner response key is `name + " " + strings.Join(args, " ")`; for `git -C <dir> rev-parse HEAD` it's exactly that string.

## T20: k3s restore
- Concurrent T17 already created k3s.go with exported `RootFlags` (not `rootFlags`); follow that convention.
- Package-level `newRunner` var in k3s.go is the injection seam — set in tests via `setupFakeRunner(t, fake)`.
- `newK3sRoot()` helper in validate_test.go needed `--confirm-context` persistent flag added for restore tests.
- backup.Manager.Restore needs these kubectl jsonpath responses to succeed: get deployments (items), get deployment replicas, get pvc volumeName, get pv local.path.
- LocalDir in testdata/infra/infra.yaml is relative (`testdata/backups`); tests must rewrite to absolute path under a tempdir fixture.
- For "no app args" mode, `resolveRestoreApps` scans `<localDir>/<ts>/<pvc>.tar.gz` filesystem to discover apps with present backups — auto-skips PVC-less apps.

## T17 — k3s install/upgrade/uninstall
- k3s package already had `RootFlags` exported, `newRunner` overridable var, `newK3sCmd(flags)` entry. Followed same pattern for install/upgrade/uninstall.
- Wired k3s into root.go via PersistentPreRunE on the k3s cobra command, copying values from the unexported `rootFlags` (cli pkg) into `k3s.RootFlags` so we don't need to modify init/update/upgrade.
- Helm client prepends `--kube-context <ctx>` (when `Context` is set) BEFORE the verb, so tests must locate release name by searching for the verb in `Args`, not by index.
- `config.MergedValueFiles` / `MergedPostRenderer` take `(*AppConfig, *InfraConfig)` (not `(Defaults, App)` as the task description suggested).
- `config.SortedByOrder` takes `*InfraConfig`, returns `[]AppConfig`.
- Reverse uninstall: build sorted list, reverse only when `--all`.
- Confirmation prompt reuses bufio.Scanner pattern from internal/cli/upgrade.go.

## F1 Plan Compliance Audit - 2026-05-11
- Read `.sisyphus/plans/easyinfra-cli.md` end-to-end and audited Must Have, Must NOT Have, and deliverables.
- Verified with `ls`, source reads, pattern searches, `go test ./...`, `go build ./...`, and `lsp_diagnostics`.
- Rejected due to forbidden `client-go` dependency string in `go.mod`/`go.sum`, `.goreleaser.yml` release owner mismatch (`guneet-xyz` vs planned `guneet`), and backup/k8s context not passed into `k8s.Client`.
