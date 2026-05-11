package k3s

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/guneet/easyinfra/pkg/backup"
	"github.com/guneet/easyinfra/pkg/config"
	"github.com/guneet/easyinfra/pkg/k8s"
	"github.com/guneet/easyinfra/pkg/paths"
	"github.com/spf13/cobra"
)

type restoreFlags struct {
	timestamp string
	yes       bool
}

func newRestoreCmd(rootF *RootFlags) *cobra.Command {
	f := &restoreFlags{}
	cmd := &cobra.Command{
		Use:   "restore [app...]",
		Short: "Restore PVCs from a previous backup",
		Long: `Restore overwrites PVC contents from a previously created backup.

With no app arguments, all apps that have backup tarballs for the chosen
timestamp are restored. When --timestamp is omitted, the latest backup is used.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRestore(cmd, rootF, f, args)
		},
	}
	cmd.Flags().StringVar(&f.timestamp, "timestamp", "", "backup timestamp to restore (default: latest)")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "skip confirmation prompt")
	return cmd
}

func runRestore(cmd *cobra.Command, rootF *RootFlags, f *restoreFlags, args []string) error {
	ctx := cmd.Context()

	cfgPath := rootF.Config
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

	runner := newRunner(cmd, rootF)
	if err := config.VerifyKubeContext(ctx, cfg, runner, rootF.ConfirmContext); err != nil {
		return err
	}

	mgr := &backup.Manager{
		Runner: runner,
		K8s:    &k8s.Client{Runner: runner},
		Cfg:    cfg.Backup,
	}

	timestamp := f.timestamp
	if timestamp == "" {
		ts, err := mgr.LatestTimestamp()
		if err != nil {
			return err
		}
		timestamp = ts
	}

	apps, err := resolveRestoreApps(cfg.Apps, args, cfg.Backup.LocalDir, timestamp)
	if err != nil {
		return err
	}
	if len(apps) == 0 {
		return fmt.Errorf("no apps with backup tarballs found in %s",
			filepath.Join(cfg.Backup.LocalDir, timestamp))
	}

	if !f.yes {
		if !confirmRestore(cmd, apps, timestamp) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Restore cancelled")
			return nil
		}
	}

	if err := mgr.Restore(ctx, apps, timestamp); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Restored %d apps from %s\n", len(apps), timestamp)
	return nil
}

func confirmRestore(cmd *cobra.Command, apps []config.AppConfig, timestamp string) bool {
	names := make([]string, len(apps))
	for i, a := range apps {
		names[i] = a.Name
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"Restore %s from %s? This will WIPE current PVC contents. [y/N] ",
		strings.Join(names, ", "), timestamp)

	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		return false
	}
	resp := strings.TrimSpace(scanner.Text())
	return resp == "y" || resp == "Y"
}

func resolveRestoreApps(allApps []config.AppConfig, requested []string, localDir, timestamp string) ([]config.AppConfig, error) {
	if len(requested) > 0 {
		byName := make(map[string]config.AppConfig, len(allApps))
		for _, a := range allApps {
			byName[a.Name] = a
		}
		out := make([]config.AppConfig, 0, len(requested))
		for _, name := range requested {
			a, ok := byName[name]
			if !ok {
				return nil, fmt.Errorf("unknown app %q (configured apps: %s)",
					name, strings.Join(configuredNames(allApps), ", "))
			}
			out = append(out, a)
		}
		return out, nil
	}

	tsDir := filepath.Join(localDir, timestamp)
	var out []config.AppConfig
	for _, app := range allApps {
		if len(app.PVCs) == 0 {
			continue
		}
		for _, pvc := range app.PVCs {
			if _, err := os.Stat(filepath.Join(tsDir, pvc+".tar.gz")); err == nil {
				out = append(out, app)
				break
			}
		}
	}
	return out, nil
}

func configuredNames(apps []config.AppConfig) []string {
	names := make([]string, len(apps))
	for i, a := range apps {
		names[i] = a.Name
	}
	return names
}
