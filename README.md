# easyinfra

> Config-driven CLI for managing k3s infrastructure via Helm.

[![CI](https://github.com/guneet-xyz/easyinfra/actions/workflows/ci.yml/badge.svg)](https://github.com/guneet-xyz/easyinfra/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/guneet-xyz/easyinfra)](https://github.com/guneet-xyz/easyinfra/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`easyinfra` reads a single `infra.yaml` from a git repo you control and turns it into Helm releases on a k3s cluster. It handles install, upgrade, uninstall, validation, and PVC backup/restore over SSH.

## Install

### One-liner (Linux, macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/guneet-xyz/easyinfra/main/install.sh | sh
```

The script detects your OS and architecture, downloads the matching binary, verifies the SHA-256 checksum, and installs to `/usr/local/bin/easyinfra` (override with `INSTALL_DIR=...`).

Supported platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.

### Manual download

Grab the archive for your platform from the [releases page](https://github.com/guneet-xyz/easyinfra/releases/latest), then verify and extract:

```sh
sha256sum -c easyinfra_<version>_<os>_<arch>.tar.gz.sha256
tar -xzf easyinfra_<version>_<os>_<arch>.tar.gz
mv easyinfra /usr/local/bin/
```

### go install

```sh
go install github.com/guneet-xyz/easyinfra/cmd/easyinfra@latest
```

## Quickstart

```sh
easyinfra init https://github.com/<you>/<infra-repo>.git
# Edit ~/.config/easyinfra/repo/infra.yaml
easyinfra k3s validate
easyinfra k3s install --all
```

## Commands

| Command | Description | Example |
|---------|-------------|---------|
| `easyinfra init <url>` | Clone infra repo to `~/.config/easyinfra/repo` | `easyinfra init https://github.com/you/infra.git` |
| `easyinfra update` | Pull latest changes in infra repo | `easyinfra update` |
| `easyinfra upgrade` | Self-upgrade the CLI binary | `easyinfra upgrade --check` |
| `easyinfra version` | Print version, commit, build date | `easyinfra version` |
| `easyinfra k3s install <app\|--all>` | Install app(s) via `helm install` | `easyinfra k3s install --all` |
| `easyinfra k3s upgrade <app\|--all>` | Upgrade app(s) via `helm upgrade` | `easyinfra k3s upgrade myapp` |
| `easyinfra k3s uninstall <app\|--all>` | Uninstall app(s) via `helm uninstall` | `easyinfra k3s uninstall --all --yes` |
| `easyinfra k3s validate [app...]` | Render charts with `helm template` | `easyinfra k3s validate` |
| `easyinfra k3s backup [app...]` | Backup PVCs over SSH/SCP | `easyinfra k3s backup` |
| `easyinfra k3s restore [app...]` | Restore PVCs from a backup snapshot | `easyinfra k3s restore --timestamp 2024-01-01_120000` |

### Global flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Override config path (default `~/.config/easyinfra/repo/infra.yaml`) |
| `--dry-run` | Print actions without executing helm or remote commands |
| `--verbose` | Stream underlying helm and SSH output |
| `--confirm-context` | Prompt for confirmation that `kubectl` current-context matches `kubeContext` |

## Configuration

`easyinfra` reads `infra.yaml` from the cloned infra repo. All paths inside the file resolve relative to `infra.yaml` itself.

```yaml
# kubeContext: required. Must match `kubectl config current-context`.
kubeContext: my-k3s-cluster

# defaults: applied to all apps unless overridden
defaults:
  postRenderer:
    command: obscuro
    args: [inject]
  valueFiles:
    - values-shared.yaml

# backup: SSH/SCP configuration for PVC backups
backup:
  remoteHost: myserver.local
  remoteUser: ubuntu          # optional, defaults to current user
  remoteTmp: /tmp/easyinfra-backups
  localDir: ~/backups/k3s

# apps: list of Helm chart deployments
apps:
  - name: caddy               # release name (also used as namespace)
    chart: apps/caddy         # path relative to infra.yaml
    namespace: caddy
    order: 1                  # lower = installed first
    valueFiles:
      - apps/caddy/values.yaml
    postRenderer:             # overrides defaults.postRenderer
      command: obscuro
      args: [inject]
    dependsOn: []             # parsed for cycle detection; ordering uses `order`
    pvcs:                     # PVC names to back up (must be spec.local.path type)
      - caddy-data
```

### Notes

- **Ordering**: apps run in ascending `order`. `dependsOn` is parsed and checked for cycles, but does not influence execution order.
- **Backup scope**: only PersistentVolumes with `spec.local.path` are supported. Other PV types are skipped with a warning.
- **Context safety**: every k3s command verifies `kubectl` is pointed at `kubeContext` before acting. Use `--confirm-context` for an interactive prompt.

## Development

```sh
git clone https://github.com/guneet-xyz/easyinfra.git
cd easyinfra
make build    # builds bin/easyinfra
make test     # unit tests with race detector
make e2e      # end-to-end tests (requires helm in PATH)
make lint     # golangci-lint
```

## License

MIT, see [LICENSE](LICENSE).
