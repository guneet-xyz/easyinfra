# easyinfra Go CLI — Migration & Bootstrap

## TL;DR

> **Quick Summary**: Build a Go CLI (`easyinfra`) that replaces three bash scripts (`deploy.sh`, `validate.sh`, `backup.sh`) with a config-driven (`infra.yaml`) tool, plus repo management (`init`, `update`), self-upgrade, multi-platform releases, install script, and CI/CD workflows.
>
> **Deliverables**:
> - `easyinfra` Go binary with cobra-based commands: `init`, `update`, `upgrade` (self), and `k3s {install,upgrade,uninstall,validate,backup,restore}`
> - `infra.yaml` schema + parser/validator
> - `install.sh` curl|sh installer (auto-detects platform)
> - GoReleaser config + GitHub Actions CI + release workflows
> - Public `github.com/guneet/easyinfra` repo (MIT license)
> - 5 platform binaries: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
> - Unit + integration + e2e test suite
> - README with usage docs
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: Wave 1 scaffolding → Wave 2 core packages → Wave 3 commands → Wave 4 release infra → Final review

---

## Context

### Original Request
Migrate scripts at `/Users/guneet/projects/infra/machines/pax/k3s/` (deploy.sh, validate.sh, backup.sh) to a Go CLI. Move app config (namespace, deployment order, post-renderer) into `infra.yaml`. Add install script, multi-platform releases, upgrade command, CI + release workflows.

### Interview Summary

**Existing scripts (source of truth for behavior to replicate)**:
- `deploy.sh`: `helm install/upgrade/uninstall <chart>`, `--atomic --wait`, namespace == chart dir name, `--post-renderer obscuro --post-renderer-args inject`
- `validate.sh`: `helm template` for every dir under `apps/`, skips library charts (`type: library` in Chart.yaml)
- `backup.sh`: kubectl-discovers PVC host paths, scales deployments to 0, SSHes to `pax`, tars `local.path`, scps back to `backups/<timestamp>/`. Hardcoded app→PVC mapping. Restore reverses, defaults to latest backup.

**Key Discussions**:
- Repo: `github.com/guneet/easyinfra`, public, MIT
- Binary: `easyinfra`, framework: cobra + viper
- Working dir: `/Users/guneet/projects/easyinfra/` (currently empty — greenfield)
- Single kube context (no multi-cluster complexity for v1)
- Infra repo cloned to `~/.config/easyinfra/repo` via `easyinfra init <git-url>`
- `infra.yaml` at root of cloned repo (NOT at `machines/pax/k3s/infra.yaml`)
- Command structure resolves naming collisions:
  - `easyinfra init` — clone infra repo
  - `easyinfra update` — git pull infra repo
  - `easyinfra upgrade` — self-upgrade CLI binary
  - `easyinfra k3s {install,upgrade,uninstall,validate,backup,restore}` — cluster ops
- Strict TDD (RED-GREEN-REFACTOR) per task
- Use `go-selfupdate` library for cross-platform self-replace
- Use system `ssh`/`scp` (delegate auth to ssh-agent/keys) for backup
- Use `helm` and `kubectl` binaries via exec.Command (NOT client-go) — matches existing script semantics, simpler, no SDK lock-in

### Metis Review

**Identified Gaps (addressed in plan)**:
- kube context: pinned in infra.yaml top-level (`kubeContext`), CLI refuses to run on mismatch unless `--confirm-context` passed
- PVC backup scope: local PVs only; clear error on non-local PVs (matches current behavior)
- Self-upgrade Windows: use `creativeprojects/go-selfupdate` library
- `dependsOn`: parsed and validated for cycles, but ordering uses `order` field only in v1 (dependsOn enforcement deferred)
- SSH auth: delegate entirely to system `ssh` (no in-Go SSH client) — uses ssh-agent / `~/.ssh/config` automatically
- Backup retention: no automatic cleanup in v1; documented in README, optional flag `--keep-last N` deferred
- Backup partial failure: keep what succeeded, log clearly, exit non-zero
- GitHub API rate limit on self-upgrade: documented in README; unauthenticated public API is sufficient

---

## Work Objectives

### Core Objective
Deliver a polished, config-driven Go CLI replacing three bash scripts, with multi-platform distribution, self-upgrade, and full CI/CD automation, ready for daily use against a single k3s cluster.

### Concrete Deliverables
- Repo: `/Users/guneet/projects/easyinfra/` initialized as Go module `github.com/guneet/easyinfra`
- Binary: `easyinfra` exposing `init`, `update`, `upgrade`, `k3s install|upgrade|uninstall|validate|backup|restore`, `version`
- Config: `infra.yaml` schema with parser + validator (lives in user's infra repo at `~/.config/easyinfra/repo/infra.yaml`)
- Installer: `install.sh` at repo root (detects OS/arch, downloads from GH Releases, installs to `/usr/local/bin` or `~/.local/bin`)
- CI: `.github/workflows/ci.yml` (lint, vet, test, build verification)
- Release: `.github/workflows/release.yml` (tag-triggered, GoReleaser-driven)
- `.goreleaser.yml` for 5-platform builds with checksums + archives
- README, LICENSE (MIT), CONTRIBUTING (basic)
- Test fixtures: `testdata/` with dummy infra repos for e2e tests

### Definition of Done
- [ ] `go build ./...` succeeds; `easyinfra --help` lists all commands
- [ ] `go test ./...` passes (unit + integration); `make e2e` passes
- [ ] CI workflow green on a test commit
- [ ] Tagging `v0.1.0` triggers release workflow producing 5 platform artifacts on GitHub Releases
- [ ] `curl -fsSL https://raw.githubusercontent.com/guneet/easyinfra/main/install.sh | sh` installs the binary on linux/amd64 + darwin/arm64 (verified via QA)
- [ ] All QA scenarios pass with evidence captured

### Must Have
- All 3 scripts' behavior reproduced (deploy/validate/backup)
- Config-as-code via `infra.yaml` at root of infra repo
- `easyinfra init <git-url>` clones infra repo to `~/.config/easyinfra/repo`
- `easyinfra update` runs `git pull` in the cloned repo
- `easyinfra upgrade` self-replaces binary from latest GitHub Release matching current platform
- Multi-platform binaries: linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64
- `install.sh` works on macOS + Linux without `bash` (POSIX-compatible `sh`)
- `kubeContext` pinned in `infra.yaml`; CLI refuses to run on context mismatch
- All commands honor `--config <path>` to override default `~/.config/easyinfra/repo/infra.yaml`
- `--dry-run` flag on destructive commands (install, upgrade, uninstall, backup, restore)
- TDD: every package has unit tests with ≥80% coverage on logic (excluding main + cobra glue)

### Must NOT Have (Guardrails)
- **MUST NOT** implement config drift detection ("is helm release in sync with infra.yaml")
- **MUST NOT** implement plugin/hook system beyond `postRenderer`
- **MUST NOT** add TUI/interactive mode (no bubbletea, no survey prompts beyond `[y/N]` confirms on destructive ops)
- **MUST NOT** enforce `dependsOn` graph in v1 (parse + cycle-check only; ordering uses `order` field)
- **MUST NOT** implement backup retention/rotation in v1 (documented manual cleanup)
- **MUST NOT** use Helm SDK or client-go — shell out to `helm` and `kubectl` binaries
- **MUST NOT** implement in-Go SSH client — delegate to system `ssh`/`scp`
- **MUST NOT** support PV types other than `spec.local.path` (clear error on others)
- **MUST NOT** add Homebrew tap, telemetry, analytics, update-check on every command, or auto-update prompts
- **MUST NOT** add Docker image, Helm chart of CLI itself, or k8s operator wrappers
- **MUST NOT** introduce non-cobra/non-viper CLI patterns (no urfave/cli mixed in)
- **MUST NOT** create config migration/versioning logic for `infra.yaml` (single schema v1)
- **MUST NOT** add features not explicitly listed in "Must Have" — every addition needs user sign-off
- **MUST NOT** generate placeholder/TODO comments in committed code
- **MUST NOT** add fake test data unless it's a clearly labeled fixture under `testdata/`

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: NO (greenfield project — Go test infrastructure created in Wave 1)
- **Automated tests**: YES (TDD)
- **Framework**: Go stdlib `testing` + `testify/require` for assertions + `testify/mock` for mocks
- **TDD flow**: RED (failing test) → GREEN (minimal impl) → REFACTOR per task
- **Coverage gate**: `go test -cover` on each package; CI fails if pkg coverage drops below 80% (excluding `cmd/easyinfra/main.go` and pure cobra wiring)

### QA Policy
Every task includes agent-executed QA scenarios. Tools by domain:
- **CLI binary execution**: `interactive_bash` (tmux) — run command, capture output, assert exit code + stdout/stderr
- **File system effects**: `bash` (ls, cat, file) — verify created files, permissions, contents
- **HTTP/GitHub API**: `bash` (curl) — verify install.sh downloads correctly, release artifacts present
- **GitHub Actions**: `bash` (`gh` CLI) — trigger workflows, fetch run status, validate outputs
- **Helm/kubectl integration**: mocked via `PATH`-injected fake binaries in `testdata/bin/` (no real cluster needed for most tests)

Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

> Maximize throughput. Target 5-8 tasks per wave.

```
Wave 1 (Foundation — start immediately, max parallel):
├── Task 1: Repo scaffolding + go.mod + Makefile + .gitignore + LICENSE [quick]
├── Task 2: cmd/easyinfra/main.go + root cobra command + version subcmd [quick]
├── Task 3: pkg/config types + infra.yaml schema definition [quick]
├── Task 4: pkg/paths — XDG/config path helpers (~/.config/easyinfra/repo) [quick]
├── Task 5: pkg/exec — exec.Command wrapper with logging + dry-run support [quick]
├── Task 6: testdata/ fixtures — mini infra.yaml + fake helm/kubectl scripts [quick]
└── Task 7: README skeleton + CONTRIBUTING + LICENSE (MIT) [quick]

Wave 2 (Core packages — depend only on Wave 1):
├── Task 8: pkg/config parser + validator (schema, kubeContext, cycle check) [unspecified-high]
├── Task 9: pkg/repo — git clone/pull wrapper for ~/.config/easyinfra/repo [unspecified-high]
├── Task 10: pkg/k8s — kubectl wrapper (current-context, get-pvc, get-pv, scale, wait) [unspecified-high]
├── Task 11: pkg/helm — helm wrapper (install, upgrade, uninstall, template) [unspecified-high]
├── Task 12: pkg/backup — SSH/SCP wrapper + tar orchestration [deep]
├── Task 13: pkg/release — GitHub API client (latest release lookup) [unspecified-high]
└── Task 14: pkg/selfupdate — wraps creativeprojects/go-selfupdate [quick]

Wave 3 (Commands — depend on Wave 2 packages):
├── Task 15: cmd init + cmd update (repo management) [unspecified-high]
├── Task 16: cmd upgrade (self-upgrade using pkg/selfupdate + pkg/release) [quick]
├── Task 17: cmd k3s install + upgrade + uninstall (uses pkg/helm + pkg/config + pkg/k8s) [deep]
├── Task 18: cmd k3s validate (helm template + schema validation) [unspecified-high]
├── Task 19: cmd k3s backup (uses pkg/backup + pkg/k8s) [deep]
├── Task 20: cmd k3s restore (uses pkg/backup + pkg/k8s) [deep]
└── Task 21: E2E test harness — runs built binary against testdata/ fixtures [unspecified-high]

Wave 4 (Distribution — depend on Wave 3 working binary):
├── Task 22: install.sh — POSIX sh installer with platform detection [unspecified-high]
├── Task 23: .goreleaser.yml — 5-platform builds, checksums, archives [unspecified-high]
├── Task 24: .github/workflows/ci.yml — lint, vet, test, build verification [quick]
├── Task 25: .github/workflows/release.yml — tag-triggered GoReleaser [quick]
├── Task 26: README full content — install/quickstart/commands/config reference [writing]
└── Task 27: First release — push tag v0.1.0, verify artifacts [quick]

Wave FINAL (Review — runs in parallel after Wave 4):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA — install.sh + every command (unspecified-high)
└── F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay
```

### Dependency Matrix

- **1-7**: no deps → unblock 8-14
- **8 (config)**: depends 3, 6 → unblocks 15, 17, 18, 19, 20, 21
- **9 (repo)**: depends 4, 5 → unblocks 15
- **10 (k8s)**: depends 5, 6 → unblocks 17, 18, 19, 20
- **11 (helm)**: depends 5, 6, 8 → unblocks 17, 18
- **12 (backup)**: depends 5, 10 → unblocks 19, 20
- **13 (release)**: depends 5 → unblocks 14, 16
- **14 (selfupdate)**: depends 13 → unblocks 16
- **15 (init/update)**: depends 8, 9 → unblocks 21
- **16 (upgrade)**: depends 13, 14 → unblocks 21
- **17 (k3s install/upgrade/uninstall)**: depends 8, 10, 11 → unblocks 21
- **18 (k3s validate)**: depends 8, 11 → unblocks 21
- **19 (k3s backup)**: depends 8, 10, 12 → unblocks 21
- **20 (k3s restore)**: depends 8, 10, 12 → unblocks 21
- **21 (e2e harness)**: depends 15, 16, 17, 18, 19, 20 → unblocks 22, 27
- **22 (install.sh)**: depends 27 (needs released binaries to test against) — *exception: build first, test E2E after Wave 4*
- **23 (goreleaser)**: depends 21 → unblocks 25, 27
- **24 (CI)**: depends 21 → can land independently
- **25 (release workflow)**: depends 23 → unblocks 27
- **26 (README)**: depends 22, 23 → unblocks 27
- **27 (first release)**: depends 22, 23, 24, 25, 26 → unblocks F3
- **F1-F4**: depend 27

### Agent Dispatch Summary

- **Wave 1** (7 tasks): T1-T7 → `quick`
- **Wave 2** (7 tasks): T8 → `unspecified-high`, T9 → `unspecified-high`, T10 → `unspecified-high`, T11 → `unspecified-high`, T12 → `deep`, T13 → `unspecified-high`, T14 → `quick`
- **Wave 3** (7 tasks): T15 → `unspecified-high`, T16 → `quick`, T17 → `deep`, T18 → `unspecified-high`, T19 → `deep`, T20 → `deep`, T21 → `unspecified-high`
- **Wave 4** (6 tasks): T22 → `unspecified-high`, T23 → `unspecified-high`, T24 → `quick`, T25 → `quick`, T26 → `writing`, T27 → `quick`
- **FINAL** (4 tasks): F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

> Implementation + Test = ONE Task. TDD: write failing test first, then minimal impl, then refactor.
> EVERY task includes Recommended Agent Profile + Parallelization + QA Scenarios.

- [x] 1. Repo scaffolding (go.mod, Makefile, .gitignore, LICENSE)

  **What to do**:
  - `cd /Users/guneet/projects/easyinfra && git init`
  - Create `go.mod`: `module github.com/guneet/easyinfra` (go 1.22+)
  - Create `Makefile` with targets: `build`, `test`, `lint`, `vet`, `e2e`, `clean`, `cover`
    - `build`: `go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o bin/easyinfra ./cmd/easyinfra`
    - `test`: `go test -race -cover ./...`
    - `lint`: `golangci-lint run` (assumes installed)
    - `e2e`: `go test -tags=e2e -race ./test/e2e/...`
  - Create `.gitignore`: `bin/`, `dist/`, `coverage.out`, `*.exe`, `.DS_Store`, `.idea/`, `.vscode/`
  - Create `LICENSE` (MIT, copyright Guneet 2026)
  - Create directory skeleton: `cmd/easyinfra/`, `internal/`, `pkg/`, `test/e2e/`, `testdata/`
  - Initial empty commit + scaffolding commit

  **Must NOT do**: No README content yet (Task 7), no CI files (Task 24), no package code (Tasks 2-14)

  **Recommended Agent Profile**:
  - **Category**: `quick` — Mechanical scaffolding, no decisions
  - **Skills**: `[]` — None needed; standard Go layout

  **Parallelization**: Wave 1 | Blocks: all other tasks | Blocked By: None

  **References**:
  - **Pattern**: Standard Go CLI layout — `https://github.com/golang-standards/project-layout`
  - **External**: cobra docs — `https://github.com/spf13/cobra`
  - **WHY**: Establishes module path consumed by every subsequent import

  **Acceptance Criteria**:
  - [ ] `go mod tidy` succeeds (no deps yet, just module declaration)
  - [ ] `make build` produces `bin/easyinfra` (will fail until Task 2; placeholder `main.go` with empty `func main(){}` to make it compile)
  - [ ] `git log` shows scaffolding commit

  **QA Scenarios**:
  ```
  Scenario: Repo compiles after scaffolding
    Tool: bash
    Preconditions: Task 1 + Task 2 placeholder exist
    Steps:
      1. cd /Users/guneet/projects/easyinfra
      2. go build ./...
      3. ls bin/easyinfra (after make build)
    Expected Result: Exit 0; binary exists
    Failure Indicators: Module path mismatch, missing dirs
    Evidence: .sisyphus/evidence/task-1-build.txt

  Scenario: Makefile targets exist
    Tool: bash
    Steps:
      1. make -n build test lint vet e2e clean cover
    Expected Result: Each target prints commands without errors
    Evidence: .sisyphus/evidence/task-1-make-targets.txt
  ```

  **Commit**: YES — `chore: scaffold repo (go.mod, Makefile, .gitignore, LICENSE)`

---

- [x] 2. cmd/easyinfra/main.go + root cobra command + version subcommand

  **What to do**:
  - `go get github.com/spf13/cobra@latest github.com/spf13/viper@latest`
  - Create `cmd/easyinfra/main.go`: build `version`, `commit`, `date` set via ldflags; calls `cmd.Execute()`
  - Create `internal/cli/root.go`: cobra root command, `Use: "easyinfra"`, `Short: "Manage k3s infra via config-as-code"`, persistent flags `--config`, `--dry-run`, `--verbose`
  - Create `internal/cli/version.go`: `version` subcommand printing `easyinfra <version> (<commit>) built <date> <runtime.GOOS>/<runtime.GOARCH>`
  - Wire version into root in `init()`

  **Must NOT do**: No `init`/`update`/`upgrade`/`k3s` subcommands yet (Tasks 15-20). No actual config loading (Task 8 wires it).

  **Recommended Agent Profile**:
  - **Category**: `quick` — Standard cobra wiring
  - **Skills**: `[]`

  **Parallelization**: Wave 1 | Blocks: 15, 16, 17, 18, 19, 20 | Blocked By: 1

  **References**:
  - cobra version pattern: `https://github.com/cli/cli/blob/trunk/cmd/gh/main.go` — ldflags injection
  - **WHY**: Every subcommand attaches to the root; ldflags populate version output for `easyinfra upgrade` checks

  **Acceptance Criteria**:
  - [ ] TDD: `internal/cli/version_test.go` asserts version subcommand prints expected format (use `cobra` test pattern: `rootCmd.SetArgs([]string{"version"}); rootCmd.SetOut(buf); rootCmd.Execute()`)
  - [ ] `go test ./internal/cli/...` PASS
  - [ ] `make build VERSION=test COMMIT=abc123 DATE=2026-05-11` produces binary; `./bin/easyinfra version` prints `easyinfra test (abc123) built 2026-05-11 darwin/arm64`
  - [ ] `./bin/easyinfra --help` lists `version` and shows `--config`, `--dry-run`, `--verbose` flags

  **QA Scenarios**:
  ```
  Scenario: version subcommand output
    Tool: interactive_bash
    Preconditions: make build VERSION=0.0.0-test COMMIT=deadbeef DATE=2026-05-11 succeeded
    Steps:
      1. tmux new-session -d -s qa-t2
      2. tmux send-keys -t qa-t2 "./bin/easyinfra version" Enter
      3. tmux capture-pane -t qa-t2 -p
    Expected Result: Output contains "easyinfra 0.0.0-test (deadbeef) built 2026-05-11"
    Evidence: .sisyphus/evidence/task-2-version.txt

  Scenario: --help output structure
    Tool: bash
    Steps:
      1. ./bin/easyinfra --help
    Expected Result: Output contains "Usage:", "Available Commands:", "version", "--config", "--dry-run"
    Evidence: .sisyphus/evidence/task-2-help.txt
  ```

  **Commit**: YES — `feat(cli): add root command and version subcommand`

---

- [x] 3. pkg/config types + infra.yaml schema definition

  **What to do**:
  - Create `pkg/config/types.go` with structs (use `yaml:"..."` tags for `gopkg.in/yaml.v3`):
    ```go
    type InfraConfig struct {
        KubeContext string         `yaml:"kubeContext"`
        Defaults    Defaults       `yaml:"defaults"`
        Backup      BackupConfig   `yaml:"backup"`
        Apps        []AppConfig    `yaml:"apps"`
    }
    type Defaults struct {
        PostRenderer *PostRenderer `yaml:"postRenderer,omitempty"`
        ValueFiles   []string      `yaml:"valueFiles,omitempty"`
    }
    type PostRenderer struct {
        Command string   `yaml:"command"`
        Args    []string `yaml:"args"`
    }
    type BackupConfig struct {
        RemoteHost string `yaml:"remoteHost"`
        RemoteUser string `yaml:"remoteUser,omitempty"` // defaults to current user via ssh
        RemoteTmp  string `yaml:"remoteTmp"`
        LocalDir   string `yaml:"localDir"`
    }
    type AppConfig struct {
        Name         string        `yaml:"name"`
        Chart        string        `yaml:"chart"`        // path relative to infra.yaml
        Namespace    string        `yaml:"namespace"`
        Order        int           `yaml:"order"`
        ValueFiles   []string      `yaml:"valueFiles,omitempty"`   // relative to infra.yaml
        PostRenderer *PostRenderer `yaml:"postRenderer,omitempty"` // overrides default
        DependsOn    []string      `yaml:"dependsOn,omitempty"`
        PVCs         []string      `yaml:"pvcs,omitempty"`
    }
    ```
  - Create `pkg/config/example_infra.yaml` (committed) showing complete example matching user's actual apps (caddy, walls, registry, etc.)

  **Must NOT do**: No parser/validator yet (Task 8). No defaults-merging logic. No CLI integration.

  **Recommended Agent Profile**:
  - **Category**: `quick` — Pure data structures
  - **Skills**: `[]`

  **Parallelization**: Wave 1 | Blocks: 8 | Blocked By: 1

  **References**:
  - User's existing `values-shared.yaml` at `/Users/guneet/projects/infra/machines/pax/k3s/values-shared.yaml` — the apps to model
  - User's existing `backup.sh` `pvcs_for_app()` cases — the PVC mappings (caddy-data, walls-postgres-data, litellm-data + litellm-postgres-data, openwebui-data, infisical-postgres-data, registry-data)
  - **WHY**: Schema must capture every fact currently in shell scripts; example file becomes the canonical reference users copy

  **Acceptance Criteria**:
  - [ ] TDD: `pkg/config/types_test.go` round-trips `example_infra.yaml` through `yaml.Unmarshal` then `yaml.Marshal` and asserts no field loss
  - [ ] Example infra.yaml contains all 11 apps from values-shared.yaml with correct namespaces
  - [ ] `go test ./pkg/config/...` PASS

  **QA Scenarios**:
  ```
  Scenario: Example infra.yaml parses to expected structure
    Tool: bash
    Steps:
      1. go test ./pkg/config/ -run TestExampleParses -v
    Expected Result: PASS, output shows "11 apps loaded"
    Evidence: .sisyphus/evidence/task-3-parse.txt

  Scenario: Round-trip preserves data
    Tool: bash
    Steps:
      1. go test ./pkg/config/ -run TestRoundTrip -v
    Expected Result: PASS, no field diff
    Evidence: .sisyphus/evidence/task-3-roundtrip.txt
  ```

  **Commit**: YES — `feat(config): add infra.yaml schema types`

---

- [x] 4. pkg/paths — XDG/config path helpers

  **What to do**:
  - Create `pkg/paths/paths.go` with functions:
    - `ConfigDir() string` — returns `$XDG_CONFIG_HOME/easyinfra` or `~/.config/easyinfra`
    - `RepoDir() string` — returns `ConfigDir()/repo`
    - `DefaultConfigPath() string` — returns `RepoDir()/infra.yaml`
    - `BinaryDir() (string, error)` — returns dir of current executable (for self-upgrade)
    - `EnsureDir(path string) error` — `os.MkdirAll` with 0755
  - Cross-platform: use `os.UserConfigDir()` for Windows compatibility (`%AppData%\easyinfra`)

  **Must NOT do**: No git logic (Task 9). No actual file IO beyond mkdir.

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**: Wave 1 | Blocks: 9, 16 | Blocked By: 1

  **References**:
  - `os.UserConfigDir()` Go stdlib — `https://pkg.go.dev/os#UserConfigDir`
  - **WHY**: Centralizes path logic so init/update/upgrade/config-loading all agree on the same locations cross-platform

  **Acceptance Criteria**:
  - [ ] TDD: `pkg/paths/paths_test.go` asserts:
    - On unix: `ConfigDir()` ends with `/easyinfra`
    - `RepoDir()` ends with `/easyinfra/repo`
    - `XDG_CONFIG_HOME` override is honored (test sets env var, asserts result)
  - [ ] Tests pass on darwin and linux (CI matrix)

  **QA Scenarios**:
  ```
  Scenario: XDG override honored
    Tool: bash
    Steps:
      1. XDG_CONFIG_HOME=/tmp/xdg-test go test ./pkg/paths/ -run TestXDGOverride -v
    Expected Result: PASS; ConfigDir() == /tmp/xdg-test/easyinfra
    Evidence: .sisyphus/evidence/task-4-xdg.txt

  Scenario: Default location
    Tool: bash
    Steps:
      1. unset XDG_CONFIG_HOME; go test ./pkg/paths/ -v
    Expected Result: PASS; ConfigDir() ends with .config/easyinfra
    Evidence: .sisyphus/evidence/task-4-default.txt
  ```

  **Commit**: YES — `feat(paths): add XDG config path helpers`

---

- [x] 5. pkg/exec — exec.Command wrapper with logging + dry-run support

  **What to do**:
  - Create `pkg/exec/runner.go`:
    ```go
    type Runner interface {
        Run(ctx context.Context, name string, args ...string) (stdout, stderr string, err error)
        RunInteractive(ctx context.Context, name string, args ...string) error
    }
    type RealRunner struct {
        DryRun, Verbose bool
        Stdout, Stderr  io.Writer
        Env             []string
    }
    type FakeRunner struct {  // tests only
        Calls     []FakeCall
        Responses map[string]FakeResponse  // keyed by "name args..."
    }
    ```
  - Dry-run: log `would run: <cmd>`, return empty stdout/stderr, nil err
  - Verbose: log `running: <cmd>` to stderr before exec
  - Inject Runner via constructor into pkg/k8s, pkg/helm, pkg/backup

  **Must NOT do**: No helm/kubectl-specific knowledge here.

  **Recommended Agent Profile**: `quick`, skills `[]`

  **Parallelization**: Wave 1 | Blocks: 9, 10, 11, 12, 13 | Blocked By: 1

  **References**:
  - Go stdlib `os/exec` — `https://pkg.go.dev/os/exec`
  - **WHY**: Single chokepoint enables dry-run, audit log, test mocking

  **Acceptance Criteria**:
  - [ ] TDD: `pkg/exec/runner_test.go` covers success, non-zero exit, dry-run skip, verbose logging, FakeRunner recording
  - [ ] `go test -race ./pkg/exec/` PASS
  - [ ] Coverage ≥90%

  **QA Scenarios**:
  ```
  Scenario: Dry-run skips actual exec
    Tool: bash
    Steps:
      1. go test ./pkg/exec/ -run TestDryRun -v
    Expected Result: PASS; output shows "would run:" log, no actual command output
    Evidence: .sisyphus/evidence/task-5-dryrun.txt

  Scenario: FakeRunner returns canned responses
    Tool: bash
    Steps:
      1. go test ./pkg/exec/ -run TestFakeRunner -v
    Expected Result: PASS
    Evidence: .sisyphus/evidence/task-5-fake.txt
  ```

  **Commit**: YES — `feat(exec): add command runner with dry-run`

---

- [x] 6. testdata/ fixtures — mini infra.yaml + fake helm/kubectl scripts

  **What to do**:
  - `testdata/infra/infra.yaml` — minimal config: 2 apps (`alpha`, `beta`), `kubeContext: test-ctx`, fake postRenderer (`echo` with args `noop`)
  - `testdata/infra/charts/alpha/{Chart.yaml, values.yaml, templates/configmap.yaml}` — minimal valid chart
  - `testdata/infra/charts/beta/{Chart.yaml, values.yaml, templates/deployment.yaml}` — minimal valid chart
  - `testdata/infra/values-shared.yaml` — shared values
  - `testdata/bin/helm` (POSIX sh) — fake helm: logs args to `$HELM_LOG` if set; supports `template` (prints canned YAML), `install`/`upgrade`/`uninstall` (exit 0), `version` (prints `v3.x`)
  - `testdata/bin/kubectl` (POSIX sh) — fake kubectl: `config current-context` → `test-ctx`; `get pvc -o jsonpath` → canned PV name; `get pv -o jsonpath` → canned local path; `scale`/`wait`/`get deployments` all exit 0 with canned output
  - `chmod +x testdata/bin/*`
  - `testdata/README.md` — usage: tests prepend `testdata/bin` to PATH

  **Must NOT do**: No real helm/kubectl invocation. Scripts must work with no external deps.

  **Recommended Agent Profile**: `quick`, skills `[]`

  **Parallelization**: Wave 1 | Blocks: 8, 10, 11, 18, 21 | Blocked By: 1

  **References**:
  - User's existing `apps/caddy/Chart.yaml` — shape to mimic
  - **WHY**: Centralized fixtures avoid drift across packages

  **Acceptance Criteria**:
  - [ ] `helm template testdata/infra/charts/alpha` succeeds with REAL helm
  - [ ] `PATH=testdata/bin:$PATH which helm` resolves to fake
  - [ ] `PATH=testdata/bin:$PATH helm template foo` prints canned YAML
  - [ ] `PATH=testdata/bin:$PATH kubectl config current-context` prints `test-ctx`

  **QA Scenarios**:
  ```
  Scenario: Real helm validates fixture charts
    Tool: bash
    Preconditions: helm installed locally
    Steps:
      1. helm template testdata/infra/charts/alpha
      2. helm template testdata/infra/charts/beta
    Expected Result: Both exit 0 with valid YAML
    Evidence: .sisyphus/evidence/task-6-real-helm.txt

  Scenario: Fake binaries respond correctly
    Tool: bash
    Steps:
      1. PATH=testdata/bin:$PATH helm template foo
      2. PATH=testdata/bin:$PATH kubectl config current-context
    Expected Result: First → canned YAML; second → "test-ctx"
    Evidence: .sisyphus/evidence/task-6-fakes.txt
  ```

  **Commit**: YES — `test: add testdata fixtures (infra.yaml + fake helm/kubectl)`

---

- [x] 7. README skeleton + CONTRIBUTING + LICENSE verification

  **What to do**:
  - Create `README.md` skeleton with sections (filled in T26):
    - Title + 1-line tagline
    - Badge placeholders (CI, Release, License)
    - Install / Quickstart / Commands / Configuration / Development / License — all with `<!-- TODO: T26 -->` markers
  - Create `CONTRIBUTING.md` (~30 lines): build/test/lint commands, conventional-commit format, direct-to-main workflow
  - Verify `LICENSE` (created in T1) is MIT with `Copyright (c) 2026 Guneet`

  **Must NOT do**: No detailed install instructions yet (no binaries). No filled command reference. T26 fills these.

  **Recommended Agent Profile**: `quick`, skills `[]`

  **Parallelization**: Wave 1 | Blocks: 26 | Blocked By: 1

  **References**:
  - **WHY**: Skeleton keeps repo presentable from commit 1

  **Acceptance Criteria**:
  - [ ] `README.md`, `CONTRIBUTING.md`, `LICENSE` exist at repo root
  - [ ] `LICENSE` first line: `MIT License`
  - [ ] README contains 6 section markers

  **QA Scenarios**:
  ```
  Scenario: Required docs exist with correct structure
    Tool: bash
    Steps:
      1. ls README.md CONTRIBUTING.md LICENSE
      2. head -1 LICENSE
      3. grep -c "^## " README.md
    Expected Result: All 3 files exist; LICENSE first line == "MIT License"; README has ≥6 sections
    Evidence: .sisyphus/evidence/task-7-docs.txt
  ```

  **Commit**: YES — `docs: add README skeleton and CONTRIBUTING`

---

- [ ] 8. pkg/config parser + validator (schema, kubeContext, cycle check)

  **What to do**:
  - `pkg/config/loader.go`:
    - `Load(path string) (*InfraConfig, error)` — reads file via `os.ReadFile`, `yaml.Unmarshal`, calls `Validate`
    - `Validate(*InfraConfig, baseDir string) error` — accumulates errors via `errors.Join`:
      - `kubeContext` non-empty
      - At least 1 app
      - Per app: `name`, `chart`, `namespace`, `order` all set
      - `chart` path exists relative to baseDir
      - `valueFiles` paths exist (per app + per defaults)
      - No duplicate `name` across apps
      - `dependsOn` references existing app names
      - `dependsOn` graph has no cycles (DFS-based, error names participating apps)
      - `backup.remoteHost` non-empty if any app has `pvcs`
    - `MergedPostRenderer(*AppConfig, *InfraConfig) *PostRenderer` — app override wins, else defaults
    - `MergedValueFiles(*AppConfig, *InfraConfig) []string` — defaults-first, then app, deduped
    - `SortedByOrder(*InfraConfig) []AppConfig` — stable sort ascending
  - `pkg/config/context.go`:
    - `VerifyKubeContext(cfg *InfraConfig, runner exec.Runner, force bool) error` — runs `kubectl config current-context`; if mismatch and !force, returns descriptive error including expected, actual, and how to use `--confirm-context`

  **Must NOT do**: No CLI integration. No `dependsOn` ordering. No defaults beyond explicit merge logic.

  **Recommended Agent Profile**: `unspecified-high`, skills `[]`

  **Parallelization**: Wave 2 | Blocks: 15, 17, 18, 19, 20, 21 | Blocked By: 3, 5, 6

  **References**:
  - User's existing `values-shared.yaml` and `AGENTS.md` for schema intent
  - `errors.Join` Go 1.20+ — `https://pkg.go.dev/errors#Join`
  - **WHY**: Gatekeeper for every command — bad config must fail fast with actionable errors

  **Acceptance Criteria**:
  - [ ] TDD: `pkg/config/loader_test.go` covers:
    - Valid `testdata/infra/infra.yaml` loads
    - Missing kubeContext → error contains "kubeContext"
    - Duplicate app name → error mentions both
    - Cycle in dependsOn → error names cycle
    - Missing chart dir → error includes path
    - VerifyKubeContext mismatch → descriptive error
    - VerifyKubeContext mismatch + force → nil
    - SortedByOrder stable for equal orders
  - [ ] Coverage ≥85% on pkg/config

  **QA Scenarios**:
  ```
  Scenario: Valid fixture loads cleanly
    Tool: bash
    Steps:
      1. go test ./pkg/config/ -run TestLoadValid -v
    Expected Result: PASS
    Evidence: .sisyphus/evidence/task-8-valid.txt

  Scenario: Cycle detection names participants
    Tool: bash
    Steps:
      1. go test ./pkg/config/ -run TestCycleDetection -v
    Expected Result: PASS; error contains "cycle: a -> b -> a"
    Evidence: .sisyphus/evidence/task-8-cycle.txt

  Scenario: kubeContext mismatch refuses
    Tool: bash
    Steps:
      1. go test ./pkg/config/ -run TestKubeContextMismatch -v
    Expected Result: PASS; error mentions both contexts
    Evidence: .sisyphus/evidence/task-8-ctx-mismatch.txt
  ```

  **Commit**: YES — `feat(config): add parser and validator`

---

- [ ] 9. pkg/repo — git clone/pull wrapper for ~/.config/easyinfra/repo

  **What to do**:
  - `pkg/repo/repo.go`:
    - `Manager{Runner exec.Runner; RepoDir string}`
    - `Clone(ctx, gitURL, branch string, force bool) error` — if `RepoDir` exists and !force → error; if force, `os.RemoveAll`; then `git clone [--branch X] gitURL RepoDir`
    - `Pull(ctx) error` — `git -C RepoDir pull --ff-only`; clear error if not a git repo or fast-forward blocked
    - `Status(ctx) (*Status, error)` — `git -C RepoDir rev-parse HEAD` + `git -C RepoDir remote get-url origin` + dirty check
    - `Exists() bool` — `RepoDir/.git` exists

  **Must NOT do**: No git library (use system git). No auto-stash. No auto-merge. Fail loudly.

  **Recommended Agent Profile**: `unspecified-high`, skills `[]`

  **Parallelization**: Wave 2 | Blocks: 15 | Blocked By: 4, 5

  **References**:
  - `git pull --ff-only` semantics — `https://git-scm.com/docs/git-pull`
  - **WHY**: System git respects user's existing config (creds, ssh keys, ssh-agent)

  **Acceptance Criteria**:
  - [ ] TDD: `pkg/repo/repo_test.go` uses FakeRunner to assert correct args + error mapping
  - [ ] Integration test (build tag `integration`) clones a tiny public repo and pulls
  - [ ] Coverage ≥85%

  **QA Scenarios**:
  ```
  Scenario: Clone uses correct git args
    Tool: bash
    Steps:
      1. go test ./pkg/repo/ -run TestCloneArgs -v
    Expected Result: PASS; FakeRunner recorded ["git","clone","--branch","main","URL","DEST"]
    Evidence: .sisyphus/evidence/task-9-clone-args.txt

  Scenario: Pull fails on non-fast-forward
    Tool: bash
    Steps:
      1. go test ./pkg/repo/ -run TestPullNonFF -v
    Expected Result: PASS; error mentions "not fast-forward"
    Evidence: .sisyphus/evidence/task-9-pull-nonff.txt

  Scenario: Real clone (integration)
    Tool: bash
    Steps:
      1. go test -tags=integration ./pkg/repo/ -run TestRealClone -v
    Expected Result: PASS; temp dir contains cloned files
    Evidence: .sisyphus/evidence/task-9-real-clone.txt
  ```

  **Commit**: YES — `feat(repo): add git clone/pull wrapper`

---

- [ ] 10. pkg/k8s — kubectl wrapper

  **What to do**:
  - `pkg/k8s/client.go`:
    - `Client{Runner exec.Runner; Context, Kubeconfig string}` — Context/Kubeconfig append `--context`/`--kubeconfig` flags when non-empty
    - `CurrentContext(ctx) (string, error)` — runs `kubectl config current-context`
    - `GetPVC(ctx, namespace, name string) (*PVC, error)` — `kubectl get pvc -n NS NAME -o jsonpath='{.spec.volumeName}'`; PVC has VolumeName field
    - `GetPVLocalPath(ctx, pvName string) (string, error)` — `kubectl get pv NAME -o jsonpath='{.spec.local.path}'`; returns descriptive error including PV type if `local.path` is empty
    - `ListDeployments(ctx, namespace string) ([]string, error)` — `kubectl get deployments -n NS -o jsonpath='{.items[*].metadata.name}'`
    - `ScaleDeployment(ctx, namespace, name string, replicas int) error` — `kubectl scale deployment NAME -n NS --replicas=N --timeout=120s`
    - `WaitForPodsDeleted(ctx, namespace string) error` — `kubectl wait --for=delete pod --all -n NS --timeout=120s`
    - `GetDeploymentReplicas(ctx, namespace, name string) (int, error)` — `kubectl get deployment NAME -n NS -o jsonpath='{.spec.replicas}'`

  **Must NOT do**: NO client-go. NO informers. NO watch. Pure shell-out via Runner.

  **Recommended Agent Profile**: `unspecified-high`, skills `[]`

  **Parallelization**: Wave 2 | Blocks: 17, 18, 19, 20 | Blocked By: 5, 6

  **References**:
  - User's existing `backup.sh` `pvc_host_path()`, `deployments_in_namespace()`, `scale_deployments()` — exact behavior to replicate
  - **WHY**: Replicating script semantics 1:1 makes migration drop-in

  **Acceptance Criteria**:
  - [ ] TDD: `pkg/k8s/client_test.go` uses FakeRunner; covers each method's args + error mapping
  - [ ] `GetPVLocalPath` returns descriptive error when path empty (mentions PV type)
  - [ ] Integration test (build tag `integration`, gated on env `EASYINFRA_E2E_KUBE`) hits real cluster (skipped in normal CI)
  - [ ] Coverage ≥85%

  **QA Scenarios**:
  ```
  Scenario: Methods invoke correct kubectl args
    Tool: bash
    Steps:
      1. go test ./pkg/k8s/ -v
    Expected Result: PASS; all method-args tests succeed
    Evidence: .sisyphus/evidence/task-10-kubectl-args.txt

  Scenario: Non-local PV produces clear error
    Tool: bash
    Steps:
      1. go test ./pkg/k8s/ -run TestNonLocalPV -v
    Expected Result: PASS; error message contains "not a local PV" or "local.path empty"
    Evidence: .sisyphus/evidence/task-10-nonlocal.txt
  ```

  **Commit**: YES — `feat(k8s): add kubectl wrapper`

---

- [ ] 11. pkg/helm — helm wrapper

  **What to do**:
  - `pkg/helm/client.go`:
    - `Client{Runner exec.Runner; Kubeconfig, Context string}`
    - `Install(ctx, opts InstallOpts) error` — builds args: `helm install RELEASE CHART -n NS --create-namespace -f F1 -f F2 --atomic --wait [--post-renderer CMD --post-renderer-args ARGS...]`
    - `Upgrade(ctx, opts InstallOpts) error` — `helm upgrade RELEASE CHART -n NS -f F1 -f F2 --atomic --wait [--post-renderer ...]`
    - `Uninstall(ctx, release, namespace string) error` — `helm uninstall RELEASE -n NS`
    - `Template(ctx, opts TemplateOpts) (string, error)` — `helm template CHART -f F1 -f F2`; returns rendered YAML
    - `IsLibraryChart(chartPath string) (bool, error)` — reads `Chart.yaml`, returns true if `type: library`
    - `InstallOpts{Release, Chart, Namespace string; ValueFiles []string; PostRenderer *config.PostRenderer; ExtraArgs []string}`
  - Multi-arg `--post-renderer-args`: pass each as separate flag occurrence per helm semantics

  **Must NOT do**: NO Helm SDK. NO chart loading via `chart.Loader`. Pure shell-out.

  **Recommended Agent Profile**: `unspecified-high`, skills `[]`

  **Parallelization**: Wave 2 | Blocks: 17, 18 | Blocked By: 5, 6, 8

  **References**:
  - User's `deploy.sh` lines 41-58 — exact helm flags to replicate
  - User's `validate.sh` — library chart skip via `grep 'type: library'`
  - **WHY**: Replicates current behavior including `--post-renderer obscuro --post-renderer-args inject`

  **Acceptance Criteria**:
  - [ ] TDD: `pkg/helm/client_test.go` uses FakeRunner; asserts:
    - Install with PostRenderer produces correct flag sequence
    - Upgrade does NOT pass `--create-namespace`
    - IsLibraryChart returns true for `testdata/infra/charts/lib/Chart.yaml` (add a library chart fixture)
  - [ ] Real-helm integration test (build tag `integration`) runs `Template` against fixtures
  - [ ] Coverage ≥85%

  **QA Scenarios**:
  ```
  Scenario: Install args match deploy.sh
    Tool: bash
    Steps:
      1. go test ./pkg/helm/ -run TestInstallArgs -v
    Expected Result: PASS; recorded args == ["install","caddy","apps/caddy","-n","caddy","--create-namespace","-f","values-shared.yaml","-f","apps/caddy/values.yaml","--atomic","--wait","--post-renderer","obscuro","--post-renderer-args","inject"]
    Evidence: .sisyphus/evidence/task-11-install-args.txt

  Scenario: Real helm template against fixture
    Tool: bash
    Preconditions: helm installed
    Steps:
      1. go test -tags=integration ./pkg/helm/ -run TestRealTemplate -v
    Expected Result: PASS; output contains valid YAML
    Evidence: .sisyphus/evidence/task-11-real-template.txt
  ```

  **Commit**: YES — `feat(helm): add helm wrapper`

---

- [ ] 12. pkg/backup — SSH/SCP wrapper + tar orchestration

  **What to do**:
  - `pkg/backup/backup.go`:
    - `Manager{Runner exec.Runner; K8s *k8s.Client; Cfg config.BackupConfig}`
    - `Run(ctx, apps []config.AppConfig) (timestamp string, err error)`:
      - Generate timestamp `YYYY-MM-DD_HHMMSS`
      - Per app: save deployment replica counts, scale all to 0, wait for pods deleted, for each PVC: resolve host path via K8s, ssh `tar czf REMOTE_TMP/TS/PVC.tar.gz -C HOST_PATH .`, then restore replicas
      - After all apps: scp -r `USER@HOST:REMOTE_TMP/TS/*` to `LOCAL_DIR/TS/`
      - ssh `rm -rf REMOTE_TMP/TS`
      - On per-app failure: log, continue, return aggregated error (errors.Join)
    - `Restore(ctx, apps []config.AppConfig, timestamp string) error`:
      - If timestamp empty: pick latest from `LOCAL_DIR` via `os.ReadDir` + sort
      - Per app: scale down, wait, scp local tar → REMOTE_TMP/TS/, ssh `rm -rf HOST_PATH/* && tar xzf REMOTE_TMP/TS/PVC.tar.gz -C HOST_PATH`, restore replicas
      - Cleanup remote tmp
    - `LatestTimestamp(localDir string) (string, error)` — sorts directory entries, returns newest

  **Must NOT do**: NO Go SSH client. NO custom auth. Use system `ssh`/`scp` via Runner. NO retention logic. NO tar parsing in Go.

  **Recommended Agent Profile**: `deep` — Many sequential steps, error-handling matters
  - **Skills**: `[]`

  **Parallelization**: Wave 2 | Blocks: 19, 20 | Blocked By: 5, 10

  **References**:
  - User's `backup.sh` `do_backup()` lines 123-159 and `do_restore()` lines 164+ — exact orchestration
  - **WHY**: Replicates behavior including replica save/restore, remote tmp cleanup

  **Acceptance Criteria**:
  - [ ] TDD: `pkg/backup/backup_test.go` uses FakeRunner; covers:
    - Backup orchestration: scale-down, tar, scp, scale-up sequence
    - Backup of 2 apps: each independent, both succeed
    - One app's tar fails: other still backed up, error returned naming failed app
    - Restore picks latest timestamp when none given
    - Restore preserves replica counts
  - [ ] Coverage ≥80%

  **QA Scenarios**:
  ```
  Scenario: Full backup orchestration
    Tool: bash
    Steps:
      1. go test ./pkg/backup/ -run TestBackupOrchestration -v
    Expected Result: PASS; FakeRunner shows correct order: get-replicas → scale 0 → wait → ssh tar → scale N → scp → ssh rm
    Evidence: .sisyphus/evidence/task-12-orchestration.txt

  Scenario: Partial failure preserves successful backups
    Tool: bash
    Steps:
      1. go test ./pkg/backup/ -run TestPartialFailure -v
    Expected Result: PASS; aggregated error names failed app; succeeded app's files survive
    Evidence: .sisyphus/evidence/task-12-partial.txt

  Scenario: Restore picks latest when no timestamp given
    Tool: bash
    Steps:
      1. go test ./pkg/backup/ -run TestRestoreLatest -v
    Expected Result: PASS
    Evidence: .sisyphus/evidence/task-12-restore-latest.txt
  ```

  **Commit**: YES — `feat(backup): add SSH/SCP/tar orchestration`

---

- [ ] 13. pkg/release — GitHub API client (latest release lookup)

  **What to do**:
  - `pkg/release/github.go`:
    - `Client{HTTPClient *http.Client; Owner, Repo string; Token string /* optional */}`
    - `LatestRelease(ctx) (*Release, error)` — GET `https://api.github.com/repos/OWNER/REPO/releases/latest`; sets `Accept: application/vnd.github+json`; if Token set, sets `Authorization: Bearer`
    - `Release{TagName, Name, Body string; Assets []Asset; PublishedAt time.Time}`
    - `Asset{Name, BrowserDownloadURL string; Size int64; ContentType string}`
    - `FindAsset(release *Release, goos, goarch string) (*Asset, error)` — matches GoReleaser naming convention `easyinfra_{goos-Title}_{x86_64|arm64}.{tar.gz|zip}`; returns error listing available assets if no match
  - Handle 403 rate-limit with descriptive error including reset time

  **Must NOT do**: NO go-github library (single endpoint, stdlib http is fine). NO release publishing (CI does that).

  **Recommended Agent Profile**: `unspecified-high`, skills `[]`

  **Parallelization**: Wave 2 | Blocks: 14, 16 | Blocked By: 5

  **References**:
  - GitHub Releases API — `https://docs.github.com/en/rest/releases/releases#get-the-latest-release`
  - GoReleaser default naming — `https://goreleaser.com/customization/archive/`
  - **WHY**: Self-upgrade needs to know what to download for current platform

  **Acceptance Criteria**:
  - [ ] TDD: `pkg/release/github_test.go` uses `httptest.Server` to mock GH API; covers:
    - Successful latest release fetch
    - 404 → clear error
    - 403 rate-limit → error mentions reset time
    - FindAsset for darwin/arm64 returns `easyinfra_Darwin_arm64.tar.gz`
    - FindAsset for windows/amd64 returns `.zip`
    - FindAsset miss returns error listing all asset names
  - [ ] Coverage ≥85%

  **QA Scenarios**:
  ```
  Scenario: Asset matching across platforms
    Tool: bash
    Steps:
      1. go test ./pkg/release/ -run TestFindAsset -v
    Expected Result: PASS; all 5 platforms map to correct assets
    Evidence: .sisyphus/evidence/task-13-asset-match.txt

  Scenario: Rate-limit error is helpful
    Tool: bash
    Steps:
      1. go test ./pkg/release/ -run TestRateLimit -v
    Expected Result: PASS; error contains reset time
    Evidence: .sisyphus/evidence/task-13-ratelimit.txt
  ```

  **Commit**: YES — `feat(release): add GitHub releases API client`

---

- [ ] 14. pkg/selfupdate — wraps creativeprojects/go-selfupdate

  **What to do**:
  - `go get github.com/creativeprojects/go-selfupdate@latest`
  - `pkg/selfupdate/updater.go`:
    - `Updater{Source selfupdate.Source; Owner, Repo string; CurrentVersion string}`
    - `Update(ctx) (newVersion string, err error)` — uses go-selfupdate's `DetectLatest` + `UpdateTo` against GH releases for `OWNER/REPO`; handles cross-platform replace including Windows
    - `Check(ctx) (latest string, hasUpdate bool, err error)` — only checks, no replace
  - Wrap library to use our pkg/release naming convention if needed; if go-selfupdate handles GoReleaser archives natively (it does), use defaults

  **Must NOT do**: NO custom binary download/replace logic. NO checksum bypass. NO version check on every command (only when `easyinfra upgrade` called).

  **Recommended Agent Profile**: `quick`, skills `[]`

  **Parallelization**: Wave 2 | Blocks: 16 | Blocked By: 13

  **References**:
  - `creativeprojects/go-selfupdate` README — `https://github.com/creativeprojects/go-selfupdate`
  - **WHY**: Battle-tested library; handles Windows file-locking via mv-then-replace

  **Acceptance Criteria**:
  - [ ] TDD: `pkg/selfupdate/updater_test.go` uses go-selfupdate's `MockSource` to assert Update succeeds and Check returns expected version
  - [ ] No real binary replacement during tests
  - [ ] Coverage ≥75% (library wrapping has limited testable surface)

  **QA Scenarios**:
  ```
  Scenario: Mock source upgrade succeeds
    Tool: bash
    Steps:
      1. go test ./pkg/selfupdate/ -run TestUpdate -v
    Expected Result: PASS; new version returned matches mock
    Evidence: .sisyphus/evidence/task-14-update.txt

  Scenario: Check without upgrade
    Tool: bash
    Steps:
      1. go test ./pkg/selfupdate/ -run TestCheckNoUpdate -v
    Expected Result: PASS; hasUpdate==false when versions match
    Evidence: .sisyphus/evidence/task-14-check.txt
  ```

  **Commit**: YES — `feat(selfupdate): wrap go-selfupdate`

---

- [ ] 15. cmd init + cmd update (repo management)

  **What to do**:
  - `internal/cli/init.go`: `easyinfra init <git-url>` cobra command
    - Flags: `--branch <name>` (default: empty → use repo default), `--force` (overwrite existing)
    - Calls `pkg/repo.Manager.Clone(ctx, url, branch, force)`
    - On success: print `Cloned <url> to <RepoDir>`
    - On error: print clear message including remediation (e.g., "use --force to overwrite")
  - `internal/cli/update.go`: `easyinfra update` cobra command
    - No required args
    - Calls `pkg/repo.Manager.Pull(ctx)`
    - On success: print `Updated <RepoDir> (HEAD: <commit>)`
    - On not-a-git-repo: print "no infra repo at <path>; run `easyinfra init <url>` first"
  - Wire both into root command in `internal/cli/root.go`

  **Must NOT do**: No `--depth` shallow clone, no `--recursive` submodules (out of scope). Don't auto-init if missing — fail with hint.

  **Recommended Agent Profile**: `unspecified-high`, skills `[]`

  **Parallelization**: Wave 3 | Blocks: 21 | Blocked By: 8, 9

  **References**:
  - **WHY**: First touchpoint for users after install; UX must be polished

  **Acceptance Criteria**:
  - [ ] TDD: `internal/cli/init_test.go` and `update_test.go` use FakeRunner; assert correct git invocations
  - [ ] `init` without args → cobra usage error (exit 1)
  - [ ] `init <url>` when RepoDir exists → error mentioning `--force`
  - [ ] `update` when RepoDir missing → error mentioning `init`

  **QA Scenarios**:
  ```
  Scenario: init clones to RepoDir
    Tool: bash
    Steps:
      1. XDG_CONFIG_HOME=/tmp/qa-t15 ./bin/easyinfra init https://github.com/octocat/Hello-World.git --branch master
      2. ls /tmp/qa-t15/easyinfra/repo/.git
    Expected Result: Exit 0; .git directory exists; output includes "Cloned"
    Evidence: .sisyphus/evidence/task-15-init.txt

  Scenario: init refuses overwrite without --force
    Tool: bash
    Preconditions: previous scenario ran
    Steps:
      1. XDG_CONFIG_HOME=/tmp/qa-t15 ./bin/easyinfra init https://github.com/octocat/Hello-World.git
    Expected Result: Exit non-zero; stderr contains "--force"
    Evidence: .sisyphus/evidence/task-15-init-force.txt

  Scenario: update pulls successfully
    Tool: bash
    Steps:
      1. XDG_CONFIG_HOME=/tmp/qa-t15 ./bin/easyinfra update
    Expected Result: Exit 0; output contains "Updated" or "Already up to date"
    Evidence: .sisyphus/evidence/task-15-update.txt
  ```

  **Commit**: YES — `feat(cmd): add init and update commands`

---

- [ ] 16. cmd upgrade (self-upgrade)

  **What to do**:
  - `internal/cli/upgrade.go`: `easyinfra upgrade` cobra command
    - Flags: `--check` (only check, don't apply), `--yes` (skip [y/N] confirmation)
    - Reads current version from `cmd/easyinfra/main.go` ldflags vars (passed via root)
    - Calls `pkg/selfupdate.Updater.Check(ctx)` first; if no update, print "Already on latest (vX.Y.Z)" and exit 0
    - If `--check`, print "Update available: vX.Y.Z → vA.B.C" and exit 0
    - Else prompt `Update easyinfra X → Y? [y/N]` (skip if `--yes`); on yes, call `Update(ctx)`; print "Upgraded to vA.B.C"
    - Owner/Repo: `guneet/easyinfra` (constants)

  **Must NOT do**: No automatic upgrade prompts on other commands. No telemetry. No version-check on startup.

  **Recommended Agent Profile**: `quick`, skills `[]`

  **Parallelization**: Wave 3 | Blocks: 21 | Blocked By: 13, 14

  **References**:
  - **WHY**: Self-upgrade is a top-tier UX feature; Windows handling already in pkg/selfupdate via go-selfupdate

  **Acceptance Criteria**:
  - [ ] TDD: `internal/cli/upgrade_test.go` mocks Updater; covers:
    - `--check` doesn't call Update
    - No-update case prints "Already on latest"
    - `--yes` skips prompt
    - Prompt rejects upgrade on "n" or empty input
  - [ ] Coverage ≥80%

  **QA Scenarios**:
  ```
  Scenario: --check doesn't apply update
    Tool: interactive_bash
    Preconditions: built binary, mock release server pointing to fake source via env
    Steps:
      1. ./bin/easyinfra upgrade --check
    Expected Result: Output contains "Update available" or "Already on latest"; binary unchanged
    Evidence: .sisyphus/evidence/task-16-check.txt

  Scenario: --yes skips prompt
    Tool: bash
    Steps:
      1. go test ./internal/cli/ -run TestUpgradeYes -v
    Expected Result: PASS; no prompt text in output
    Evidence: .sisyphus/evidence/task-16-yes.txt
  ```

  **Commit**: YES — `feat(cmd): add upgrade (self-upgrade) command`

---

- [ ] 17. cmd k3s install + upgrade + uninstall

  **What to do**:
  - `internal/cli/k3s/k3s.go`: parent `k3s` cobra command (subcommand group)
  - `internal/cli/k3s/install.go`: `easyinfra k3s install <app|--all>`
    - Flags: `--all` (install all apps in `order` ascending), `--confirm-context` (skip context mismatch error)
    - Loads config via `pkg/config.Load`, calls `VerifyKubeContext`
    - For each app (sorted by Order): builds `helm.InstallOpts` from merged value files + post-renderer; calls `helm.Client.Install`
    - Honors `--dry-run` from root (passed to Runner)
  - `internal/cli/k3s/upgrade.go`: same shape, calls `helm.Client.Upgrade`
  - `internal/cli/k3s/uninstall.go`: `easyinfra k3s uninstall <app|--all>`
    - For `--all`: prompts `[y/N]` unless `--yes`; uninstalls in reverse order
    - Calls `helm.Client.Uninstall`

  **Must NOT do**: NO parallel installs (sequential by `order`). NO `dependsOn` enforcement. NO health checks beyond what `--atomic --wait` provides. NO automatic backup before uninstall.

  **Recommended Agent Profile**: `deep` — Most complex command set, integrates many packages
  - **Skills**: `[]`

  **Parallelization**: Wave 3 | Blocks: 21 | Blocked By: 8, 10, 11

  **References**:
  - User's `deploy.sh` lines 41-67 — exact behavior
  - **WHY**: Core feature — replaces the most-used script

  **Acceptance Criteria**:
  - [ ] TDD: `internal/cli/k3s/install_test.go`, `upgrade_test.go`, `uninstall_test.go` cover:
    - Install single app: helm args correct, post-renderer applied
    - Install --all: order respected
    - Uninstall --all: reverse order
    - kubeContext mismatch without --confirm-context: refuses
    - Unknown app name: clear error listing known apps
    - Dry-run: no actual helm calls (FakeRunner records "would run")
  - [ ] Coverage ≥80%

  **QA Scenarios**:
  ```
  Scenario: Install single app dry-run shows correct command
    Tool: bash
    Steps:
      1. PATH=testdata/bin:$PATH ./bin/easyinfra --config testdata/infra/infra.yaml --dry-run k3s install alpha
    Expected Result: Output contains "would run: helm install alpha testdata/infra/charts/alpha -n alpha --create-namespace ..."
    Evidence: .sisyphus/evidence/task-17-install-dryrun.txt

  Scenario: Install --all respects order
    Tool: bash
    Steps:
      1. PATH=testdata/bin:$PATH ./bin/easyinfra --config testdata/infra/infra.yaml --dry-run k3s install --all
    Expected Result: alpha installed before beta (or whichever order is set in fixture)
    Evidence: .sisyphus/evidence/task-17-install-all.txt

  Scenario: Unknown app errors clearly
    Tool: bash
    Steps:
      1. ./bin/easyinfra --config testdata/infra/infra.yaml k3s install nonexistent
    Expected Result: Exit non-zero; stderr lists known apps "alpha, beta"
    Evidence: .sisyphus/evidence/task-17-unknown-app.txt

  Scenario: kubeContext mismatch refuses
    Tool: bash
    Steps:
      1. PATH=testdata/bin-wrong-ctx:$PATH ./bin/easyinfra --config testdata/infra/infra.yaml k3s install alpha
    Expected Result: Exit non-zero; stderr mentions both contexts and --confirm-context
    Evidence: .sisyphus/evidence/task-17-ctx.txt
  ```

  **Commit**: YES — `feat(cmd): add k3s install/upgrade/uninstall`

---

- [ ] 18. cmd k3s validate

  **What to do**:
  - `internal/cli/k3s/validate.go`: `easyinfra k3s validate [app...]`
    - Loads config, calls `Validate` (schema check)
    - For each app (or filtered by args): if library chart, skip with "SKIP: <name> — library"; else `helm.Client.Template`
    - Aggregates errors; returns non-zero if any failed
    - Output format: `OK:   alpha`, `FAIL: beta`, summary `2/3 charts validated`

  **Must NOT do**: NO `kubectl --dry-run=server` validation (matches current script). NO Kustomize, NO yamllint integration.

  **Recommended Agent Profile**: `unspecified-high`, skills `[]`

  **Parallelization**: Wave 3 | Blocks: 21 | Blocked By: 8, 11

  **References**:
  - User's `validate.sh` — exact behavior including library-chart skip
  - **WHY**: Confidence check before install; replaces shell script 1:1

  **Acceptance Criteria**:
  - [ ] TDD: `internal/cli/k3s/validate_test.go` covers:
    - Valid fixture: all charts validate, exit 0
    - Filter by app name: only that one validated
    - Library chart: skipped with SKIP message
    - Invalid chart: exit 1, FAIL line present, summary correct
  - [ ] Coverage ≥85%

  **QA Scenarios**:
  ```
  Scenario: Validate all charts in fixture
    Tool: bash
    Preconditions: real helm installed
    Steps:
      1. ./bin/easyinfra --config testdata/infra/infra.yaml k3s validate
    Expected Result: Exit 0; output "OK: alpha", "OK: beta", summary "2/2 charts validated"
    Evidence: .sisyphus/evidence/task-18-validate-all.txt

  Scenario: Filter to one app
    Tool: bash
    Steps:
      1. ./bin/easyinfra --config testdata/infra/infra.yaml k3s validate alpha
    Expected Result: Exit 0; only "OK: alpha"; no mention of beta
    Evidence: .sisyphus/evidence/task-18-validate-one.txt

  Scenario: Invalid chart fails
    Tool: bash
    Preconditions: testdata/infra-invalid/ fixture with broken template
    Steps:
      1. ./bin/easyinfra --config testdata/infra-invalid/infra.yaml k3s validate
    Expected Result: Exit non-zero; "FAIL:" line present
    Evidence: .sisyphus/evidence/task-18-validate-fail.txt
  ```

  **Commit**: YES — `feat(cmd): add k3s validate`

---

- [ ] 19. cmd k3s backup

  **What to do**:
  - `internal/cli/k3s/backup.go`: `easyinfra k3s backup [app...]`
    - No args = all apps with non-empty `pvcs`; with args = filter
    - Loads config, verifies kubeContext
    - Calls `pkg/backup.Manager.Run(ctx, apps)`
    - On success: prints `Backup complete: <localDir>/<timestamp>` and `ls -lh` of result
    - On partial failure: prints succeeded apps, lists failed, exits non-zero

  **Must NOT do**: NO retention/rotation. NO compression options (always tar.gz). NO encryption (delegates to obscuro / out of scope).

  **Recommended Agent Profile**: `deep`, skills `[]`

  **Parallelization**: Wave 3 | Blocks: 21 | Blocked By: 8, 10, 12

  **References**:
  - User's `backup.sh do_backup()` — exact orchestration
  - **WHY**: Critical for cluster recovery; must be reliable

  **Acceptance Criteria**:
  - [ ] TDD: `internal/cli/k3s/backup_test.go` covers:
    - All-apps backup: each app processed
    - Filtered backup: only listed apps
    - App without pvcs: skipped with note
    - Partial failure: exit non-zero, succeeded apps listed
  - [ ] Coverage ≥80%

  **QA Scenarios**:
  ```
  Scenario: Backup dry-run shows orchestration
    Tool: bash
    Steps:
      1. PATH=testdata/bin:$PATH ./bin/easyinfra --config testdata/infra/infra.yaml --dry-run k3s backup
    Expected Result: Output shows would-run sequence: kubectl get → scale 0 → wait → ssh tar → scale N → scp
    Evidence: .sisyphus/evidence/task-19-backup-dryrun.txt

  Scenario: App without PVCs skipped
    Tool: bash
    Preconditions: testdata fixture has one app with empty pvcs list
    Steps:
      1. PATH=testdata/bin:$PATH ./bin/easyinfra --config testdata/infra/infra.yaml --dry-run k3s backup
    Expected Result: Output contains "skip: <app> — no pvcs"
    Evidence: .sisyphus/evidence/task-19-skip.txt
  ```

  **Commit**: YES — `feat(cmd): add k3s backup`

---

- [ ] 20. cmd k3s restore

  **What to do**:
  - `internal/cli/k3s/restore.go`: `easyinfra k3s restore [app...] --timestamp <ts>`
    - Flags: `--timestamp <ts>` (default: latest), `--yes` (skip confirmation)
    - No app args = all apps with backup tars in `<localDir>/<ts>/`
    - Loads config, verifies kubeContext
    - Prompts `Restore <apps> from <ts>? This will WIPE current PVC contents. [y/N]` unless `--yes`
    - Calls `pkg/backup.Manager.Restore(ctx, apps, timestamp)`
    - On success: prints `Restored <N> apps from <ts>`

  **Must NOT do**: NO partial PVC restore (always full). NO incremental restore. NO timestamp validation beyond directory existence.

  **Recommended Agent Profile**: `deep`, skills `[]`

  **Parallelization**: Wave 3 | Blocks: 21 | Blocked By: 8, 10, 12

  **References**:
  - User's `backup.sh do_restore()` — exact behavior
  - **WHY**: Recovery path must be foolproof; explicit confirmation prevents accidents

  **Acceptance Criteria**:
  - [ ] TDD: `internal/cli/k3s/restore_test.go` covers:
    - Restore with explicit timestamp
    - Restore picks latest when --timestamp omitted
    - Prompt rejects on "n" / empty
    - --yes skips prompt
    - Missing timestamp dir: clear error
  - [ ] Coverage ≥80%

  **QA Scenarios**:
  ```
  Scenario: Restore latest dry-run
    Tool: bash
    Preconditions: testdata/backups/2026-05-11_120000/ exists with fake tars
    Steps:
      1. PATH=testdata/bin:$PATH ./bin/easyinfra --config testdata/infra/infra.yaml --dry-run k3s restore --yes
    Expected Result: Output shows would-run scp + ssh tar xzf for latest timestamp
    Evidence: .sisyphus/evidence/task-20-restore-latest.txt

  Scenario: Missing timestamp errors clearly
    Tool: bash
    Steps:
      1. ./bin/easyinfra --config testdata/infra/infra.yaml k3s restore --timestamp 9999-99-99 --yes
    Expected Result: Exit non-zero; stderr contains the missing timestamp path
    Evidence: .sisyphus/evidence/task-20-missing-ts.txt
  ```

  **Commit**: YES — `feat(cmd): add k3s restore`

---

- [ ] 21. E2E test harness

  **What to do**:
  - `test/e2e/e2e_test.go` (build tag `e2e`):
    - `TestMain` builds `bin/easyinfra` once via `go build`
    - Helpers: `runEasyinfra(t, args ...) (stdout, stderr string, exitCode int)` — exec.Command with PATH including `testdata/bin`
    - Tests:
      - `TestVersion` — runs `version`, asserts output format
      - `TestInitFresh` — temp XDG_CONFIG_HOME, runs `init` against tiny public repo, asserts clone
      - `TestInitExistingRefuses` — second init errors
      - `TestInitForceReplaces` — `init --force` succeeds
      - `TestUpdate` — after init, `update` succeeds
      - `TestK3sValidate` — `k3s validate --config testdata/infra/infra.yaml` exit 0
      - `TestK3sInstallDryRun` — verifies emitted helm command
      - `TestK3sUninstallAllReverse` — verifies reverse order
      - `TestK3sBackupDryRun` — verifies orchestration
      - `TestK3sRestoreDryRun` — verifies orchestration
      - `TestKubeContextMismatch` — fixture with wrong context refuses
      - `TestUnknownAppError` — clear error
  - Each test owns a temp dir (XDG_CONFIG_HOME); cleanup in t.Cleanup
  - `make e2e` runs `go test -tags=e2e ./test/e2e/...`

  **Must NOT do**: NO real cluster required. NO real network beyond `init` test (gated on `EASYINFRA_E2E_NETWORK` env if needed; otherwise use local fixture repo via `git daemon` or `file://` URL).

  **Recommended Agent Profile**: `unspecified-high`, skills `[]`

  **Parallelization**: Wave 3 | Blocks: 22, 27 | Blocked By: 15, 16, 17, 18, 19, 20

  **References**:
  - Go testing with build tags — `https://pkg.go.dev/cmd/go#hdr-Build_constraints`
  - **WHY**: Catches integration bugs unit tests miss; runs in CI

  **Acceptance Criteria**:
  - [ ] `make e2e` exit 0 on dev machine
  - [ ] All 12+ E2E tests pass
  - [ ] No flakes on 3 consecutive runs

  **QA Scenarios**:
  ```
  Scenario: E2E suite passes
    Tool: bash
    Steps:
      1. make e2e
    Expected Result: Exit 0; all tests PASS
    Evidence: .sisyphus/evidence/task-21-e2e.txt

  Scenario: E2E suite stable across runs
    Tool: bash
    Steps:
      1. for i in 1 2 3; do make e2e || echo "FAIL on run $i"; done
    Expected Result: 3 successes, no FAIL
    Evidence: .sisyphus/evidence/task-21-stability.txt
  ```

  **Commit**: YES — `test: add e2e harness against testdata fixtures`

---

- [ ] 22. install.sh — POSIX sh installer with platform detection

  **What to do**:
  - Create `install.sh` at repo root, POSIX-compatible (`#!/bin/sh`, no bashisms):
    - Detect OS: `uname -s` → linux/darwin/Windows fail (point to releases page)
    - Detect arch: `uname -m` → x86_64→amd64, aarch64/arm64→arm64; fail on others
    - Resolve GoReleaser asset name: `easyinfra_${OS_TITLE}_${ARCH_GORELEASER}.tar.gz` (e.g., `easyinfra_Linux_x86_64.tar.gz`, `easyinfra_Darwin_arm64.tar.gz`)
    - Resolve version: default `latest`, override via `EASYINFRA_VERSION` env var or `--version vX.Y.Z` flag
    - Download URL: `https://github.com/guneet/easyinfra/releases/download/${VERSION}/${ASSET}` (or `/latest/download/` for latest)
    - Download checksums.txt, verify sha256 of asset
    - Extract via `tar -xzf` to temp dir
    - Determine install dir: `INSTALL_DIR` env (default: `/usr/local/bin` if writable, else `~/.local/bin`)
    - `mv binary` to install dir, `chmod +x`
    - Print: `easyinfra <version> installed to <path>` and run `easyinfra version` to confirm
    - Flags: `--dry-run` (print steps, no download), `--version vX.Y.Z`, `--install-dir <path>`, `--help`
  - Use `curl` if available, else `wget`; error if neither

  **Must NOT do**: NO bash-specific syntax (test on `dash` for portability). NO root install without explicit user choice. NO modifying PATH/shell rc files. NO Windows support (print message pointing to manual download).

  **Recommended Agent Profile**: `unspecified-high` — Shell portability is fiddly
  - **Skills**: `[]`

  **Parallelization**: Wave 4 | Blocks: 26, 27 | Blocked By: None for skeleton (testing requires released artifacts in T27)

  **References**:
  - GoReleaser default archive naming: `https://goreleaser.com/customization/archive/`
  - install.sh portability checklist: shellcheck — `https://www.shellcheck.net/`
  - Examples: `rustup-init.sh`, `https://get.helm.sh/get`
  - **WHY**: Most users will install via `curl|sh`; portability bugs = lost users

  **Acceptance Criteria**:
  - [ ] `shellcheck install.sh` exit 0 (run with `--shell=sh`)
  - [ ] `sh install.sh --dry-run` on macOS arm64 prints `Darwin_arm64.tar.gz`
  - [ ] `sh install.sh --dry-run` on linux amd64 prints `Linux_x86_64.tar.gz`
  - [ ] `sh install.sh --help` prints usage
  - [ ] After T27 release: `sh install.sh --version v0.1.0` actually installs and `easyinfra version` reports `v0.1.0`

  **QA Scenarios**:
  ```
  Scenario: shellcheck passes
    Tool: bash
    Steps:
      1. shellcheck --shell=sh install.sh
    Expected Result: Exit 0
    Evidence: .sisyphus/evidence/task-22-shellcheck.txt

  Scenario: Dry-run on macOS detects platform
    Tool: bash
    Steps:
      1. sh install.sh --dry-run
    Expected Result: Output contains "Darwin_arm64" (on M-series Mac)
    Evidence: .sisyphus/evidence/task-22-dryrun-mac.txt

  Scenario: Dry-run in Linux Docker container
    Tool: bash
    Steps:
      1. docker run --rm -v "$PWD/install.sh:/install.sh" alpine:latest sh /install.sh --dry-run
    Expected Result: Output contains "Linux_x86_64"
    Evidence: .sisyphus/evidence/task-22-dryrun-linux.txt

  Scenario: Real install (gated on T27 completion)
    Tool: bash
    Preconditions: v0.1.0 released
    Steps:
      1. INSTALL_DIR=/tmp/qa-t22 sh install.sh --version v0.1.0
      2. /tmp/qa-t22/easyinfra version
    Expected Result: Step 2 prints "easyinfra v0.1.0"
    Evidence: .sisyphus/evidence/task-22-real-install.txt
  ```

  **Commit**: YES — `chore: add install.sh POSIX installer`

---

- [ ] 23. .goreleaser.yml — 5-platform builds, checksums, archives

  **What to do**:
  - Create `.goreleaser.yml`:
    ```yaml
    version: 2
    project_name: easyinfra
    before:
      hooks:
        - go mod tidy
        - go test ./...
    builds:
      - id: easyinfra
        main: ./cmd/easyinfra
        binary: easyinfra
        env: [CGO_ENABLED=0]
        goos: [linux, darwin, windows]
        goarch: [amd64, arm64]
        ignore:
          - {goos: windows, goarch: arm64}
        ldflags:
          - -s -w
          - -X main.version={{.Version}}
          - -X main.commit={{.ShortCommit}}
          - -X main.date={{.Date}}
    archives:
      - id: default
        formats: [tar.gz]
        format_overrides:
          - {goos: windows, formats: [zip]}
        name_template: '{{ .ProjectName }}_{{- title .Os }}_{{- if eq .Arch "amd64" }}x86_64{{- else }}{{ .Arch }}{{- end }}'
        files: [README.md, LICENSE]
    checksum:
      name_template: 'checksums.txt'
      algorithm: sha256
    snapshot:
      version_template: '{{ incpatch .Version }}-next'
    changelog:
      sort: asc
      filters:
        exclude: ['^docs:', '^test:', '^chore:', '^ci:']
    release:
      github: {owner: guneet, name: easyinfra}
      draft: false
      prerelease: auto
    ```
  - Verify locally: `goreleaser release --snapshot --clean --skip=publish` produces `dist/` with 5 archives + checksums

  **Must NOT do**: NO Homebrew tap config. NO Docker images. NO nfpm (deb/rpm). NO scoop. Keep release artifacts minimal.

  **Recommended Agent Profile**: `unspecified-high`, skills `[]`

  **Parallelization**: Wave 4 | Blocks: 25, 27 | Blocked By: 21 (e2e must pass before adding to release pipeline)

  **References**:
  - GoReleaser docs — `https://goreleaser.com/customization/`
  - **WHY**: Single source of truth for all release artifacts

  **Acceptance Criteria**:
  - [ ] `goreleaser check` exit 0
  - [ ] Local snapshot build produces 5 archives (linux amd64, linux arm64, darwin amd64, darwin arm64, windows amd64) + checksums.txt
  - [ ] Each archive contains `easyinfra` binary, README.md, LICENSE
  - [ ] Binary in archive: `./bin-test/easyinfra version` (extracted) prints expected ldflags

  **QA Scenarios**:
  ```
  Scenario: goreleaser check passes
    Tool: bash
    Steps:
      1. goreleaser check
    Expected Result: Exit 0
    Evidence: .sisyphus/evidence/task-23-check.txt

  Scenario: Snapshot produces 5 archives
    Tool: bash
    Steps:
      1. goreleaser release --snapshot --clean --skip=publish
      2. ls dist/*.tar.gz dist/*.zip dist/checksums.txt | wc -l
    Expected Result: 4 tar.gz + 1 zip + 1 checksums = 6 files
    Evidence: .sisyphus/evidence/task-23-snapshot.txt

  Scenario: Extracted binary reports version
    Tool: bash
    Steps:
      1. tar -xzf dist/easyinfra_Darwin_arm64.tar.gz -C /tmp/qa-t23/
      2. /tmp/qa-t23/easyinfra version
    Expected Result: Output contains "easyinfra" + snapshot version
    Evidence: .sisyphus/evidence/task-23-extracted.txt
  ```

  **Commit**: YES — `chore: add .goreleaser.yml for 5-platform builds`

---

- [ ] 24. .github/workflows/ci.yml — lint, vet, test, build verification

  **What to do**:
  - Create `.github/workflows/ci.yml`:
    ```yaml
    name: CI
    on:
      push: {branches: [main]}
      pull_request:
    jobs:
      lint:
        runs-on: ubuntu-latest
        steps:
          - uses: actions/checkout@v4
          - uses: actions/setup-go@v5
            with: {go-version: '1.22', cache: true}
          - uses: golangci/golangci-lint-action@v6
            with: {version: latest}
      test:
        runs-on: ${{ matrix.os }}
        strategy:
          matrix:
            os: [ubuntu-latest, macos-latest]
        steps:
          - uses: actions/checkout@v4
          - uses: actions/setup-go@v5
            with: {go-version: '1.22', cache: true}
          - run: go mod tidy && git diff --exit-code go.mod go.sum
          - run: go vet ./...
          - run: go test -race -cover -coverprofile=coverage.out ./...
          - run: go tool cover -func=coverage.out | tail -1
      e2e:
        runs-on: ubuntu-latest
        needs: [test]
        steps:
          - uses: actions/checkout@v4
          - uses: actions/setup-go@v5
            with: {go-version: '1.22', cache: true}
          - uses: azure/setup-helm@v4
            with: {version: latest}
          - run: make e2e
      build:
        runs-on: ubuntu-latest
        needs: [test]
        steps:
          - uses: actions/checkout@v4
          - uses: actions/setup-go@v5
            with: {go-version: '1.22', cache: true}
          - uses: goreleaser/goreleaser-action@v6
            with: {args: release --snapshot --clean --skip=publish}
      shellcheck:
        runs-on: ubuntu-latest
        steps:
          - uses: actions/checkout@v4
          - run: shellcheck --shell=sh install.sh
    ```
  - Add `.golangci.yml`: enable `errcheck`, `govet`, `staticcheck`, `revive`, `gocyclo` (max 15), `misspell`, `unused`

  **Must NOT do**: NO codecov upload (out of scope). NO matrix on Go versions (lock to 1.22). NO Windows runner (Linux + macOS sufficient).

  **Recommended Agent Profile**: `quick`, skills `[]`

  **Parallelization**: Wave 4 | Blocks: None (lands independently) | Blocked By: 21

  **References**:
  - golangci-lint config — `https://golangci-lint.run/usage/configuration/`
  - **WHY**: Every push gets validated; release confidence

  **Acceptance Criteria**:
  - [ ] `gh workflow view CI` (after push) shows green run
  - [ ] All 5 jobs (lint, test on ubuntu+macos, e2e, build, shellcheck) succeed
  - [ ] `golangci-lint run` locally exit 0

  **QA Scenarios**:
  ```
  Scenario: golangci-lint passes locally
    Tool: bash
    Steps:
      1. golangci-lint run
    Expected Result: Exit 0
    Evidence: .sisyphus/evidence/task-24-lint.txt

  Scenario: CI workflow green on push
    Tool: bash
    Preconditions: pushed to main
    Steps:
      1. gh run list --workflow=ci.yml --limit 1 --json conclusion,status
    Expected Result: status=completed, conclusion=success
    Evidence: .sisyphus/evidence/task-24-ci-run.txt
  ```

  **Commit**: YES — `ci: add CI workflow (lint, test, build)`

---

- [ ] 25. .github/workflows/release.yml — tag-triggered GoReleaser

  **What to do**:
  - Create `.github/workflows/release.yml`:
    ```yaml
    name: Release
    on:
      push:
        tags: ['v*']
    permissions:
      contents: write
    jobs:
      goreleaser:
        runs-on: ubuntu-latest
        steps:
          - uses: actions/checkout@v4
            with: {fetch-depth: 0}
          - uses: actions/setup-go@v5
            with: {go-version: '1.22', cache: true}
          - uses: goreleaser/goreleaser-action@v6
            with: {args: release --clean}
            env:
              GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    ```

  **Must NOT do**: NO manual workflow_dispatch (tag-only). NO signing (cosign etc.) for v1. NO release notes editing post-publish.

  **Recommended Agent Profile**: `quick`, skills `[]`

  **Parallelization**: Wave 4 | Blocks: 27 | Blocked By: 23

  **References**:
  - GoReleaser GitHub Action — `https://github.com/goreleaser/goreleaser-action`
  - **WHY**: Pure automation; tag → released artifacts

  **Acceptance Criteria**:
  - [ ] Workflow file syntactically valid (`actionlint .github/workflows/release.yml` exit 0)
  - [ ] After T27 push tag: workflow runs and creates GH Release with 5 archives + checksums

  **QA Scenarios**:
  ```
  Scenario: actionlint passes
    Tool: bash
    Steps:
      1. actionlint .github/workflows/release.yml
    Expected Result: Exit 0
    Evidence: .sisyphus/evidence/task-25-actionlint.txt

  Scenario: Release workflow runs on tag (verified in T27)
    Tool: bash
    Steps:
      1. gh run list --workflow=release.yml --limit 1 --json conclusion,status
    Expected Result: After T27 tag push, conclusion=success
    Evidence: .sisyphus/evidence/task-25-release-run.txt
  ```

  **Commit**: YES — `ci: add release workflow (tag-triggered)`

---

- [ ] 26. README full content — install/quickstart/commands/config reference

  **What to do**:
  - Replace placeholders in `README.md` (skeleton from T7):
    - **Install** section:
      - One-liner: `curl -fsSL https://raw.githubusercontent.com/guneet/easyinfra/main/install.sh | sh`
      - Manual: link to releases page with platform download instructions
      - go install: `go install github.com/guneet/easyinfra/cmd/easyinfra@latest`
      - Verifying checksums (instructions for sha256sum)
    - **Quickstart**:
      - `easyinfra init https://github.com/<you>/<infra-repo>.git`
      - Edit `~/.config/easyinfra/repo/infra.yaml` (link to schema reference below)
      - `easyinfra k3s validate`
      - `easyinfra k3s install --all`
    - **Commands** — table: command | description | example, covering: init, update, upgrade, version, k3s install, k3s upgrade, k3s uninstall, k3s validate, k3s backup, k3s restore
    - **Configuration** — full `infra.yaml` reference: every field documented, example for caddy + walls + registry from user's actual setup
    - **Development** — clone, `make build`, `make test`, `make e2e`, `make lint`
    - **License** — MIT
  - Add badges (top): CI status, latest release version, license
  - Verify all internal anchor links work; link install.sh raw URL is correct

  **Must NOT do**: NO marketing fluff. NO comparisons to other tools. NO TODO/coming soon sections.

  **Recommended Agent Profile**: `writing` — Documentation-heavy
  - **Skills**: `[]`

  **Parallelization**: Wave 4 | Blocks: 27 | Blocked By: 22, 23

  **References**:
  - User's own `apps/` charts and `values-shared.yaml` — for realistic config example
  - Plan section "infra.yaml schema" — for reference docs
  - **WHY**: README is the project's front door; must be accurate and complete

  **Acceptance Criteria**:
  - [ ] No `<!-- TODO -->` markers remain
  - [ ] `markdownlint README.md` exit 0
  - [ ] All sections present per skeleton
  - [ ] Example config in README matches the actual schema (parses with `easyinfra k3s validate --config /tmp/example.yaml`)

  **QA Scenarios**:
  ```
  Scenario: README has no placeholders
    Tool: bash
    Steps:
      1. ! grep -E '(TODO|FIXME|placeholder|<!-- TODO)' README.md
    Expected Result: Exit 0 (no matches)
    Evidence: .sisyphus/evidence/task-26-no-placeholders.txt

  Scenario: Example infra.yaml from README parses
    Tool: bash
    Steps:
      1. awk '/^```yaml$/,/^```$/' README.md | grep -v '^```' > /tmp/readme-example.yaml
      2. ./bin/easyinfra k3s validate --config /tmp/readme-example.yaml
    Expected Result: Exit 0 (or, if charts referenced are absent, schema-only error — acceptable)
    Evidence: .sisyphus/evidence/task-26-example-parses.txt
  ```

  **Commit**: YES — `docs: complete README with install/usage/config reference`

---

- [ ] 27. First release — push tag v0.1.0, verify artifacts

  **What to do**:
  - Verify everything: `make lint test e2e` clean
  - `git tag -a v0.1.0 -m "Initial release"`
  - `git push origin v0.1.0`
  - Wait for release workflow: `gh run watch`
  - Verify GH Release: `gh release view v0.1.0` shows 5 archives + checksums.txt
  - Test install.sh end-to-end:
    - macOS: `INSTALL_DIR=/tmp/release-test sh install.sh --version v0.1.0`
    - Linux (Docker): `docker run --rm alpine sh -c "apk add curl && curl -fsSL https://raw.githubusercontent.com/guneet/easyinfra/main/install.sh | sh -s -- --version v0.1.0 --install-dir /tmp"`
  - Both must result in working `easyinfra version`

  **Must NOT do**: NO release notes hand-editing (auto-generated from changelog is fine). NO v1.0.0 (this is initial; v0.1.0 signals pre-stable).

  **Recommended Agent Profile**: `quick`, skills `[]`

  **Parallelization**: Wave 4 | Blocks: F1, F3 | Blocked By: 22, 23, 24, 25, 26

  **References**:
  - SemVer — `https://semver.org/`
  - **WHY**: Closes the loop; proves the entire release pipeline works end-to-end

  **Acceptance Criteria**:
  - [ ] `gh release view v0.1.0` exists, has 5 binary archives + checksums.txt
  - [ ] `gh run list --workflow=release.yml --limit 1 -q '.[0].conclusion'` == "success"
  - [ ] install.sh on macOS installs and `easyinfra version` reports `v0.1.0`
  - [ ] install.sh in Alpine Docker installs and reports `v0.1.0`

  **QA Scenarios**:
  ```
  Scenario: Release artifacts present
    Tool: bash
    Steps:
      1. gh release view v0.1.0 --json assets -q '.assets[].name' | sort
    Expected Result: 6 lines: 4 tar.gz, 1 zip (windows), 1 checksums.txt
    Evidence: .sisyphus/evidence/task-27-release-assets.txt

  Scenario: install.sh on macOS works against released v0.1.0
    Tool: bash
    Preconditions: macOS host (this dev machine)
    Steps:
      1. INSTALL_DIR=/tmp/qa-t27-mac sh install.sh --version v0.1.0
      2. /tmp/qa-t27-mac/easyinfra version
    Expected Result: Step 2 outputs "easyinfra v0.1.0 (...)"
    Evidence: .sisyphus/evidence/task-27-install-mac.txt

  Scenario: install.sh in Alpine Linux container works
    Tool: bash
    Steps:
      1. docker run --rm -v "$PWD/install.sh:/install.sh" alpine sh -c "apk add --no-cache curl tar && INSTALL_DIR=/tmp sh /install.sh --version v0.1.0 && /tmp/easyinfra version"
    Expected Result: Output contains "easyinfra v0.1.0"
    Evidence: .sisyphus/evidence/task-27-install-alpine.txt

  Scenario: Checksum verification
    Tool: bash
    Steps:
      1. cd /tmp && curl -fsSL https://github.com/guneet/easyinfra/releases/download/v0.1.0/checksums.txt -o checksums.txt
      2. curl -fsSLO https://github.com/guneet/easyinfra/releases/download/v0.1.0/easyinfra_Darwin_arm64.tar.gz
      3. shasum -a 256 -c <(grep Darwin_arm64 checksums.txt)
    Expected Result: "OK" line
    Evidence: .sisyphus/evidence/task-27-checksum.txt
  ```

  **Commit**: YES — `release: cut v0.1.0` (tag commit; no file changes needed beyond tag)

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read this plan end-to-end. For each "Must Have": verify implementation exists (read file, run binary command, check artifact). For each "Must NOT Have": grep codebase for forbidden patterns (e.g., `client-go`, `bubbletea`, drift detection logic) — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test -race -cover ./...`. Review all Go files for: empty error handling, `panic()` in non-init code, `fmt.Println` instead of structured logging, dead code, AI-slop patterns (over-commented, generic names like `result`/`data`/`item`, premature abstraction). Verify ≥80% coverage on logic packages.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Coverage [pkg→%] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Build binary fresh. Execute EVERY QA scenario from EVERY task. Then end-to-end smoke: `easyinfra version` → `easyinfra init <test-repo>` (against a public test repo) → `easyinfra update` → `easyinfra k3s validate` (against fixture) → `easyinfra k3s install <app> --dry-run`. Test install.sh on macOS + Linux (via Docker) downloading the released v0.1.0 binary. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Smoke [PASS/FAIL] | install.sh [macOS PASS, Linux PASS] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance per task. Detect cross-task contamination (Task N touching Task M's files). Flag unaccounted changes (files in git that aren't claimed by any task).
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

One commit per task, conventional commit format. All directly to `main`.

- T1: `chore: scaffold repo (go.mod, Makefile, .gitignore, LICENSE)`
- T2: `feat(cli): add root command and version subcommand`
- T3: `feat(config): add infra.yaml schema types`
- T4: `feat(paths): add XDG config path helpers`
- T5: `feat(exec): add command runner with dry-run`
- T6: `test: add testdata fixtures (infra.yaml + fake helm/kubectl)`
- T7: `docs: add README skeleton and CONTRIBUTING`
- T8: `feat(config): add parser and validator`
- T9: `feat(repo): add git clone/pull wrapper`
- T10: `feat(k8s): add kubectl wrapper`
- T11: `feat(helm): add helm wrapper`
- T12: `feat(backup): add SSH/SCP/tar orchestration`
- T13: `feat(release): add GitHub releases API client`
- T14: `feat(selfupdate): wrap go-selfupdate`
- T15: `feat(cmd): add init and update commands`
- T16: `feat(cmd): add upgrade (self-upgrade) command`
- T17: `feat(cmd): add k3s install/upgrade/uninstall`
- T18: `feat(cmd): add k3s validate`
- T19: `feat(cmd): add k3s backup`
- T20: `feat(cmd): add k3s restore`
- T21: `test: add e2e harness against testdata fixtures`
- T22: `chore: add install.sh POSIX installer`
- T23: `chore: add .goreleaser.yml for 5-platform builds`
- T24: `ci: add CI workflow (lint, test, build)`
- T25: `ci: add release workflow (tag-triggered)`
- T26: `docs: complete README with install/usage/config reference`
- T27: `release: cut v0.1.0`

---

## Success Criteria

### Verification Commands
```bash
# Build & test
go build ./...                                           # exit 0
go vet ./...                                             # exit 0
go test -race -cover ./...                               # all PASS, ≥80% coverage on logic pkgs
golangci-lint run                                        # exit 0

# Binary smoke
./bin/easyinfra version                                  # prints version + commit + date
./bin/easyinfra --help                                   # lists init, update, upgrade, k3s, version
./bin/easyinfra k3s --help                               # lists install, upgrade, uninstall, validate, backup, restore

# Config validation against fixture
./bin/easyinfra k3s validate --config testdata/infra/infra.yaml   # exit 0

# Release artifacts (after tagging v0.1.0)
gh release view v0.1.0 --json assets -q '.assets[].name' | wc -l  # 5 binaries + 1 checksums = 6+
curl -fsSL https://github.com/guneet/easyinfra/releases/latest/download/easyinfra_Linux_x86_64.tar.gz | tar tz   # contains 'easyinfra'

# install.sh
sh install.sh --dry-run                                  # detects platform, prints planned URL
```

### Final Checklist
- [ ] All "Must Have" items implemented and verified
- [ ] All "Must NOT Have" items absent (grep-verified)
- [ ] All unit + integration + e2e tests pass
- [ ] CI workflow green
- [ ] v0.1.0 tagged and released with 5 platform artifacts
- [ ] install.sh works on macOS + Linux
- [ ] All QA scenarios pass with evidence captured
- [ ] All 4 final verification agents APPROVE
- [ ] User explicitly oks the work
