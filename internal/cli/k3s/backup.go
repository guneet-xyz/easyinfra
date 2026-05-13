// Package k3s contains commands for managing k3s infrastructure.
package k3s

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/guneet-xyz/easyinfra/pkg/backup"
	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/k8s"
	"github.com/guneet-xyz/easyinfra/pkg/paths"
	"github.com/spf13/cobra"
)

func newBackupCmd(flags *RootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup [app...]",
		Short: "Back up PVCs for all (or selected) apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackup(cmd, flags, args)
		},
	}
	cmd.AddCommand(newBackupRecoverCmd(flags))
	cmd.AddCommand(newBackupListCmd(flags))
	cmd.AddCommand(newBackupPruneCmd(flags))
	return cmd
}

func newBackupListCmd(flags *RootFlags) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local backups",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBackupList(cmd, flags, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON instead of a text table")
	return cmd
}

func runBackupList(cmd *cobra.Command, flags *RootFlags, asJSON bool) error {
	cfgPath := flags.Config
	if cfgPath == "" {
		p, err := paths.DefaultConfigPath()
		if err != nil {
			return fmt.Errorf("resolving config path: %w", err)
		}
		cfgPath = p
	}

	cfg, err := config.LoadV2(cfgPath)
	if err != nil {
		return err
	}

	var entries []backup.Entry
	if cfg.Backup.LocalDir != "" {
		entries, err = backup.List(cfg.Backup.LocalDir)
		if err != nil {
			return err
		}
	}

	out := cmd.OutOrStdout()

	if asJSON {
		if entries == nil {
			entries = []backup.Entry{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	if len(entries) == 0 {
		_, _ = fmt.Fprintln(out, "No backups found.")
		return nil
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "TIMESTAMP\tAPPS\tSIZE\tREPLICAS"); err != nil {
		return err
	}
	for _, e := range entries {
		apps := strings.Join(e.Apps, ",")
		if apps == "" {
			apps = "-"
		}
		replicas := "no"
		if e.HasReplicas {
			replicas = "yes"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Timestamp, apps, humanSize(e.SizeBytes), replicas); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func newBackupRecoverCmd(flags *RootFlags) *cobra.Command {
	var appFilter string
	cmd := &cobra.Command{
		Use:   "recover <timestamp>",
		Short: "Re-apply replica counts saved during a previous backup",
		Long: `Recover reads <localDir>/<timestamp>/replicas.json and re-issues the kubectl
scale commands needed to restore deployments to their pre-backup replica
counts. Use this after a backup whose scale-up step failed (the backup
directory will contain a "state" marker file with content "scale-up-failed").`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupRecover(cmd, flags, args[0], appFilter)
		},
	}
	cmd.Flags().StringVar(&appFilter, "app", "", "only recover deployments in this app's namespace")
	return cmd
}

func runBackupRecover(cmd *cobra.Command, flags *RootFlags, ts, appFilter string) error {
	ctx := cmd.Context()

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

	runner := newRunner(cmd, flags)
	if err := config.VerifyKubeContext(ctx, cfg, runner, flags.ConfirmContext); err != nil {
		return err
	}

	var filter []string
	if appFilter != "" {
		filter = append(filter, appFilter)
	}

	if err := backup.Recover(ctx, cfg.Backup.LocalDir, ts, runner, filter...); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Recovery complete: %s/%s\n", cfg.Backup.LocalDir, ts)
	return nil
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

func newBackupPruneCmd(flags *RootFlags) *cobra.Command {
	var (
		keep      int
		olderThan string
		dryRun    bool
	)
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete old local backups according to a retention policy",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBackupPrune(cmd, flags, keep, olderThan, dryRun)
		},
	}
	cmd.Flags().IntVar(&keep, "keep", 0, "keep this many newest backups (overrides config)")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "delete backups older than this duration, e.g. 720h (overrides config)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list candidates without deleting")
	return cmd
}

func runBackupPrune(cmd *cobra.Command, flags *RootFlags, keep int, olderThan string, dryRun bool) error {
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

	policy := backup.PrunePolicy{DryRun: dryRun}

	if keep > 0 {
		policy.KeepN = keep
	}

	if olderThan != "" {
		d, err := time.ParseDuration(olderThan)
		if err != nil {
			return fmt.Errorf("parsing older-than %q: %w", olderThan, err)
		}
		policy.OlderThan = d
	}

	if policy.KeepN == 0 && policy.OlderThan == 0 {
		return errors.New("no retention policy: pass --keep or --older-than, or configure backup.retention")
	}

	pruned, err := backup.Prune(cfg.Backup.LocalDir, policy)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(pruned) == 0 {
		_, _ = fmt.Fprintln(out, "No backups to prune.")
		return nil
	}

	verb := "Pruned"
	if dryRun {
		verb = "Would prune"
	}
	_, _ = fmt.Fprintf(out, "%s %d backup(s):\n", verb, len(pruned))
	for _, ts := range pruned {
		_, _ = fmt.Fprintln(out, "  "+ts)
	}
	return nil
}
