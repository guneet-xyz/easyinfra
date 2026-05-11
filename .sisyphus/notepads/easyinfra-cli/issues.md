## T21 (E2E harness) findings

- CLI silences errors: `internal/cli/root.go` sets `SilenceErrors: true` and
  `internal/cli/cli.go` `Execute` discards the returned error before exiting.
  Net effect: when a command fails, the user sees only a non-zero exit code
  and zero stderr/stdout. E2E tests for `TestKubeContextMismatch` and
  `TestUnknownAppError` therefore can only assert exit code, not message
  content. Recommend a follow-up task to either remove `SilenceErrors` or
  print the returned error in `cli.Execute` (e.g.
  `fmt.Fprintln(os.Stderr, err)`).
- `--dry-run` causes `kubectl config current-context` to print "would run: ..."
  rather than executing the fake kubectl on PATH, so context verification
  fails with an empty current context. E2E tests therefore pass
  `--confirm-context` alongside `--dry-run` for k3s install/upgrade/uninstall/
  backup/restore. Document or consider whether `--dry-run` should imply or
  auto-confirm context.
- `k3s backup --dry-run` exits non-zero because the fake kubectl returns no
  PV info in dry-run mode (it isn't actually executed). Test only asserts
  orchestration commands (ssh/kubectl/scp) appear in stdout.
- `k3s restore` requires a tarball matching `--timestamp` under the
  configured `backup.localDir`. E2E test creates a fake
  `testdata/backups/<ts>/alpha-data.tar.gz` and cleans it up via `t.Cleanup`.

## F3 QA Findings (2026-05-11)

### BUG 1: Dry-run breaks on kubeContext check
Scenarios 5-8 (dry-run k3s install/uninstall/backup) fail because:
- `pkg/exec/runner.go` RealRunner.Run in DryRun mode returns empty stdout/stderr
- `pkg/config/context.go` VerifyKubeContext compares `""` (dry-run output) to `cfg.KubeContext` ("test-ctx")
- Mismatch causes early exit BEFORE any helm/ssh `would run:` lines are emitted
- Workaround: pass `--confirm-context`
- Fix: skip context check when `--dry-run` OR have dry-run runner return canned context value

### BUG 2: Errors silently swallowed
Scenario 9 (`k3s install nonexistent`) exits 1 but stderr is empty.
- `internal/cli/root.go` sets `SilenceErrors: true`
- `internal/cli/cli.go` Execute() does `os.Exit(1)` without printing the error
- User sees no message about unknown app or known apps list
- Fix: print err to stderr in Execute() before os.Exit(1), e.g. `fmt.Fprintln(os.Stderr, "Error:", err)`
