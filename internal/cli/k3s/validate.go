package k3s

import (
	"fmt"
	"path/filepath"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/helm"
	"github.com/guneet-xyz/easyinfra/pkg/paths"
	"github.com/spf13/cobra"
)

func newValidateCmd(flags *RootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "validate [app...]",
		Short: "Render each chart with helm template to validate it",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd, flags, args)
		},
	}
}

func runValidate(cmd *cobra.Command, flags *RootFlags, args []string) error {
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
	baseDir := filepath.Dir(cfgPath)

	apps, err := selectApps(cfg, args)
	if err != nil {
		return err
	}

	runner := newRunner(cmd, flags)
	client := &helm.Client{Runner: runner}

	out := cmd.OutOrStdout()
	ctx := cmd.Context()

	total := len(apps)
	passed := 0
	var failed []string

	for _, app := range apps {
		chartPath := filepath.Join(baseDir, app.Chart)

		isLib, err := client.IsLibraryChart(chartPath)
		if err != nil {
			_, _ = fmt.Fprintf(out, "FAIL: %s — %v\n", app.Name, err)
			failed = append(failed, app.Name)
			continue
		}
		if isLib {
			_, _ = fmt.Fprintf(out, "SKIP: %s — library\n", app.Name)
			total--
			continue
		}

		valueFiles := config.MergedValueFiles(&app, cfg)
		resolved := make([]string, len(valueFiles))
		for i, vf := range valueFiles {
			resolved[i] = filepath.Join(baseDir, vf)
		}

		if _, err := client.Template(ctx, chartPath, resolved); err != nil {
			_, _ = fmt.Fprintf(out, "FAIL: %s — %v\n", app.Name, err)
			failed = append(failed, app.Name)
			continue
		}
		_, _ = fmt.Fprintf(out, "OK:   %s\n", app.Name)
		passed++
	}

	_, _ = fmt.Fprintf(out, "%d/%d charts validated\n", passed, total)

	if len(failed) > 0 {
		return fmt.Errorf("validation failed for: %v", failed)
	}
	return nil
}

func selectApps(cfg *config.InfraConfig, names []string) ([]config.AppConfig, error) {
	sorted := config.SortedByOrder(cfg)
	if len(names) == 0 {
		return sorted, nil
	}
	wanted := make(map[string]bool, len(names))
	for _, n := range names {
		wanted[n] = true
	}
	var out []config.AppConfig
	seen := make(map[string]bool)
	for _, app := range sorted {
		if wanted[app.Name] {
			out = append(out, app)
			seen[app.Name] = true
		}
	}
	for _, n := range names {
		if !seen[n] {
			return nil, fmt.Errorf("unknown app: %s", n)
		}
	}
	return out, nil
}
