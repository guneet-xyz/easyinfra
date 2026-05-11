// Package k3s contains commands for managing k3s infrastructure.
package k3s

import (
	"errors"
	"fmt"
	"strings"

	"github.com/guneet-xyz/easyinfra/pkg/backup"
	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/k8s"
	"github.com/guneet-xyz/easyinfra/pkg/paths"
	"github.com/spf13/cobra"
)

func newBackupCmd(flags *RootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "backup [app...]",
		Short: "Back up PVCs for all (or selected) apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackup(cmd, flags, args)
		},
	}
}

func runBackup(cmd *cobra.Command, flags *RootFlags, args []string) error {
	cfgPath := flags.Config
	if cfgPath == "" {
		p, err := paths.DefaultConfigPath()
		if err != nil {
			return fmt.Errorf("resolving config path: %w", err)
		}
		cfgPath = p
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	selected, err := selectApps(cfg, args)
	if err != nil {
		return err
	}

	runner := newRunner(cmd, flags)
	ctx := cmd.Context()

	if err := config.VerifyKubeContext(ctx, cfg, runner, flags.ConfirmContext); err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	var toBackup []config.AppConfig
	for _, app := range selected {
		if len(app.PVCs) == 0 {
			_, _ = fmt.Fprintf(out, "skip: %s — no pvcs\n", app.Name)
			continue
		}
		toBackup = append(toBackup, app)
	}

	if len(toBackup) == 0 {
		_, _ = fmt.Fprintln(out, "No apps to back up.")
		return nil
	}

	mgr := &backup.Manager{
		Runner: runner,
		K8s:    &k8s.Client{Runner: runner},
		Cfg:    cfg.Backup,
	}

	timestamp, runErr := mgr.Run(ctx, toBackup)
	if runErr != nil {
		failed := failedAppsFromError(runErr)
		var succeeded, failedList []string
		for _, app := range toBackup {
			if failed[app.Name] {
				failedList = append(failedList, app.Name)
			} else {
				succeeded = append(succeeded, app.Name)
			}
		}
		if len(succeeded) > 0 {
			_, _ = fmt.Fprintf(out, "Succeeded: %s\n", strings.Join(succeeded, ", "))
		}
		if len(failedList) > 0 {
			_, _ = fmt.Fprintf(out, "Failed: %s\n", strings.Join(failedList, ", "))
		}
		return runErr
	}

	_, _ = fmt.Fprintf(out, "Backup complete: %s/%s\n", cfg.Backup.LocalDir, timestamp)
	return nil
}

// failedAppsFromError extracts app names from a joined error produced by
// backup.Manager.Run, which wraps per-app errors as "backup <name>: ...".
func failedAppsFromError(err error) map[string]bool {
	failed := map[string]bool{}
	collect := func(e error) {
		msg := e.Error()
		const prefix = "backup "
		if !strings.HasPrefix(msg, prefix) {
			return
		}
		rest := strings.TrimPrefix(msg, prefix)
		i := strings.Index(rest, ":")
		if i <= 0 {
			return
		}
		failed[rest[:i]] = true
	}
	type multi interface{ Unwrap() []error }
	if u, ok := err.(multi); ok {
		for _, e := range u.Unwrap() {
			collect(e)
		}
		return failed
	}
	// Fall through: single error, walk Unwrap chain.
	for e := err; e != nil; e = errors.Unwrap(e) {
		collect(e)
	}
	return failed
}
