// Package doctor provides preflight checks for easyinfra.
package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	execpkg "github.com/guneet-xyz/easyinfra/pkg/exec"
)

// Status represents the outcome of a check.
type Status string

const (
	// StatusOK indicates the check passed.
	StatusOK Status = "ok"
	// StatusWarn indicates the check passed with a warning.
	StatusWarn Status = "warn"
	// StatusFail indicates the check failed.
	StatusFail Status = "fail"
)

// Result is the outcome of a single check.
type Result struct {
	Name    string
	Status  Status
	Message string
	Detail  string
}

// Check is a single preflight check.
type Check interface {
	Name() string
	Run(ctx context.Context) Result
}

// Report aggregates check results.
type Report struct {
	Checks []Result
	Failed bool
}

// Run executes all checks and returns a Report.
func Run(ctx context.Context, checks []Check) Report {
	r := Report{}
	for _, c := range checks {
		res := c.Run(ctx)
		r.Checks = append(r.Checks, res)
		if res.Status == StatusFail {
			r.Failed = true
		}
	}
	return r
}

// BinaryCheck verifies a binary is on PATH.
type BinaryCheck struct {
	Binary      string   // e.g. "helm"
	VersionArgs []string // e.g. ["version", "--short"]
	InstallURL  string   // shown in Detail when missing
}

// Name returns the check identifier.
func (c BinaryCheck) Name() string { return "binary:" + c.Binary }

// Run executes the binary presence check.
func (c BinaryCheck) Run(ctx context.Context) Result {
	path, err := exec.LookPath(c.Binary)
	if err != nil {
		return Result{
			Name:    c.Name(),
			Status:  StatusFail,
			Message: c.Binary + " not found on PATH",
			Detail:  "Install from: " + c.InstallURL,
		}
	}
	return Result{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: c.Binary + " found",
		Detail:  path,
	}
}

// KubeContextCheck verifies the kube context is reachable.
// When NoCluster is true, the check is skipped (returns warn).
type KubeContextCheck struct {
	Context   string
	NoCluster bool
	Runner    execpkg.Runner
}

// Name returns the check identifier.
func (c KubeContextCheck) Name() string { return "kube-context" }

// Run executes the kube context reachability check.
func (c KubeContextCheck) Run(ctx context.Context) Result {
	if c.NoCluster {
		return Result{
			Name:    c.Name(),
			Status:  StatusWarn,
			Message: "kube-context check skipped (--no-cluster)",
		}
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, stderr, err := c.Runner.Run(tctx, "kubectl", "--context", c.Context, "get", "ns", "-o", "name")
	if err != nil {
		return Result{
			Name:    c.Name(),
			Status:  StatusWarn,
			Message: "kube-context " + c.Context + " not reachable",
			Detail:  strings.TrimSpace(stderr),
		}
	}
	return Result{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: "kube-context " + c.Context + " reachable",
	}
}

// ConfigCheck verifies the infra.yaml loads and validates.
type ConfigCheck struct {
	Path string
}

// Name returns the check identifier.
func (c ConfigCheck) Name() string { return "config" }

// Run executes the config load and validation check.
func (c ConfigCheck) Run(ctx context.Context) Result {
	_, err := config.LoadV2(c.Path)
	if err != nil {
		return Result{
			Name:    c.Name(),
			Status:  StatusFail,
			Message: "config load failed",
			Detail:  err.Error(),
		}
	}
	return Result{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: "config valid: " + c.Path,
	}
}

// ChartPathsCheck verifies all app chart directories exist.
type ChartPathsCheck struct {
	Cfg     *config.InfraConfigV2
	BaseDir string
}

// Name returns the check identifier.
func (c ChartPathsCheck) Name() string { return "chart-paths" }

// Run executes the chart paths existence check.
func (c ChartPathsCheck) Run(ctx context.Context) Result {
	var missing []string
	for _, app := range c.Cfg.Apps {
		chartPath := filepath.Join(c.BaseDir, app.Chart)
		if _, err := os.Stat(chartPath); err != nil {
			missing = append(missing, app.Chart)
		}
	}
	if len(missing) > 0 {
		return Result{
			Name:    c.Name(),
			Status:  StatusFail,
			Message: fmt.Sprintf("%d chart path(s) missing", len(missing)),
			Detail:  strings.Join(missing, ", "),
		}
	}
	return Result{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: fmt.Sprintf("all %d chart paths exist", len(c.Cfg.Apps)),
	}
}

// BackupConfigCheck verifies backup config is sane when apps have PVCs.
type BackupConfigCheck struct {
	Cfg *config.InfraConfigV2
}

// Name returns the check identifier.
func (c BackupConfigCheck) Name() string { return "backup-config" }

// Run executes the backup configuration sanity check.
func (c BackupConfigCheck) Run(ctx context.Context) Result {
	hasPVCs := false
	for _, app := range c.Cfg.Apps {
		if len(app.PVCs) > 0 {
			hasPVCs = true
			break
		}
	}
	if hasPVCs && c.Cfg.Backup.RemoteHost == "" {
		return Result{
			Name:    c.Name(),
			Status:  StatusFail,
			Message: "backup.remoteHost required when apps have PVCs",
		}
	}
	if !hasPVCs {
		return Result{
			Name:    c.Name(),
			Status:  StatusOK,
			Message: "no PVCs configured (backup not required)",
		}
	}
	return Result{
		Name:    c.Name(),
		Status:  StatusOK,
		Message: "backup config valid (remoteHost: " + c.Cfg.Backup.RemoteHost + ")",
	}
}

// DefaultChecks returns the standard set of checks for a given config.
// noCluster skips the kube-context reachability check.
func DefaultChecks(cfg *config.InfraConfigV2, baseDir string, runner execpkg.Runner, noCluster bool) []Check {
	return []Check{
		BinaryCheck{Binary: "helm", VersionArgs: []string{"version", "--short"}, InstallURL: "https://helm.sh/docs/intro/install/"},
		BinaryCheck{Binary: "kubectl", VersionArgs: []string{"version", "--client", "--short"}, InstallURL: "https://kubernetes.io/docs/tasks/tools/"},
		BinaryCheck{Binary: "ssh", InstallURL: "https://www.openssh.com/"},
		BinaryCheck{Binary: "scp", InstallURL: "https://www.openssh.com/"},
		BinaryCheck{Binary: "git", InstallURL: "https://git-scm.com/downloads"},
		KubeContextCheck{Context: cfg.Cluster.KubeContext, NoCluster: noCluster, Runner: runner},
		ChartPathsCheck{Cfg: cfg, BaseDir: baseDir},
		BackupConfigCheck{Cfg: cfg},
	}
}
