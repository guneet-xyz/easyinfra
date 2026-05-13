package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

// AppOperation describes the backup work for a single application.
type AppOperation struct {
	AppName     string
	Namespace   string
	Deployments []string
	PVCs        []string
	RemotePath  string
	LocalTarget string
}

// Operation is the full plan of backup work to perform.
type Operation struct {
	Apps      []AppOperation
	Timestamp string
}

// Plan builds an Operation describing the backup work for the given apps.
// If apps is empty or nil, all apps from cfg are included. Apps with no PVCs
// are skipped. The Plan is a pure data structure and does not perform any
// side effects.
func Plan(cfg *config.InfraConfigV2, apps []string) *Operation {
	if cfg == nil {
		return &Operation{Timestamp: time.Now().Format("2006-01-02_150405")}
	}
	timestamp := time.Now().Format("2006-01-02_150405")
	remoteTmpTS := cfg.Backup.RemoteTmp + "/" + timestamp
	localDir := filepath.Join(cfg.Backup.LocalDir, timestamp)

	filter := make(map[string]bool, len(apps))
	for _, a := range apps {
		filter[a] = true
	}

	op := &Operation{Timestamp: timestamp}
	for _, app := range cfg.Apps {
		if len(filter) > 0 && !filter[app.Name] {
			continue
		}
		if len(app.PVCs) == 0 {
			continue
		}
		op.Apps = append(op.Apps, AppOperation{
			AppName:     app.Name,
			Namespace:   app.Namespace,
			Deployments: nil, // resolved at Execute time from cluster
			PVCs:        append([]string(nil), app.PVCs...),
			RemotePath:  remoteTmpTS,
			LocalTarget: localDir,
		})
	}
	return op
}

// Execute runs the planned Operation against the given runner. When dryRun is
// true the plan is printed to stdout and the runner is not invoked at all.
func Execute(ctx context.Context, op *Operation, runner exec.Runner, dryRun bool) error {
	return ExecuteWith(ctx, op, runner, dryRun, os.Stdout, "", "")
}

// ExecuteWith is like Execute but lets the caller supply the writer used for
// dry-run output and the SSH remote target/local dir overrides. It is mainly
// useful for tests.
func ExecuteWith(ctx context.Context, op *Operation, runner exec.Runner, dryRun bool, out io.Writer, remoteTarget, localDirOverride string) error {
	if op == nil {
		return nil
	}
	if out == nil {
		out = os.Stdout
	}
	if dryRun {
		fmt.Fprintf(out, "backup plan (timestamp=%s):\n", op.Timestamp)
		for _, app := range op.Apps {
			fmt.Fprintf(out, "  app=%s namespace=%s\n", app.AppName, app.Namespace)
			fmt.Fprintf(out, "    remote: %s\n", app.RemotePath)
			fmt.Fprintf(out, "    local:  %s\n", app.LocalTarget)
			for _, pvc := range app.PVCs {
				fmt.Fprintf(out, "    pvc:    %s\n", pvc)
			}
		}
		return nil
	}

	// Non-dry-run execution path. The full execution lives in Manager.Run for
	// historical reasons; this helper intentionally only handles the dry-run
	// preview so callers can drive Manager.Run for real work. A non-dry-run
	// call without a Manager is treated as a no-op error so that callers must
	// route real execution through Manager.Run (preserving existing behavior).
	if remoteTarget == "" && localDirOverride == "" {
		return fmt.Errorf("backup.Execute: non-dry-run requires Manager.Run; use Manager for real execution")
	}
	for _, app := range op.Apps {
		for _, pvc := range app.PVCs {
			tarCmd := fmt.Sprintf("tar czf %s/%s.tar.gz -C '/tmp/%s' .", app.RemotePath, pvc, pvc)
			if _, _, err := runner.Run(ctx, "ssh", remoteTarget, tarCmd); err != nil {
				return err
			}
		}
	}
	return nil
}
