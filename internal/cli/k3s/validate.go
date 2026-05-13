package k3s

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/helm"
	"github.com/guneet-xyz/easyinfra/pkg/paths"
	"github.com/spf13/cobra"
)

type validateResult struct {
	App    string `json:"app"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func newValidateCmd(flags *RootFlags) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "validate [app...]",
		Short: "Render each chart with helm template to validate it",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd, flags, args, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON output")
	return cmd
}

func runValidate(cmd *cobra.Command, flags *RootFlags, args []string, asJSON bool) error {
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
	baseDir := filepath.Dir(cfgPath)

	apps, err := selectAppsV2(cfg, args)
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
	var results []validateResult

	for _, app := range apps {
		chartPath := filepath.Join(baseDir, app.Chart)

		isLib, err := client.IsLibraryChart(chartPath)
		if err != nil {
			if asJSON {
				results = append(results, validateResult{
					App:    app.Name,
					Status: "fail",
					Error:  err.Error(),
				})
			} else {
				_, _ = fmt.Fprintf(out, "FAIL: %s — %v\n", app.Name, err)
			}
			failed = append(failed, app.Name)
			continue
		}
		if isLib {
			if asJSON {
				results = append(results, validateResult{
					App:    app.Name,
					Status: "skip",
					Error:  "library chart",
				})
			} else {
				_, _ = fmt.Fprintf(out, "SKIP: %s — library\n", app.Name)
			}
			total--
			continue
		}

		valueFiles := config.MergedValueFilesV2(&app, cfg)
		resolved := make([]string, len(valueFiles))
		for i, vf := range valueFiles {
			resolved[i] = filepath.Join(baseDir, vf)
		}

		if _, err := client.Template(ctx, chartPath, resolved); err != nil {
			if asJSON {
				results = append(results, validateResult{
					App:    app.Name,
					Status: "fail",
					Error:  err.Error(),
				})
			} else {
				_, _ = fmt.Fprintf(out, "FAIL: %s — %v\n", app.Name, err)
			}
			failed = append(failed, app.Name)
			continue
		}
		if asJSON {
			results = append(results, validateResult{
				App:    app.Name,
				Status: "ok",
			})
		} else {
			_, _ = fmt.Fprintf(out, "OK:   %s\n", app.Name)
		}
		passed++
	}

	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	_, _ = fmt.Fprintf(out, "%d/%d charts validated\n", passed, total)

	if len(failed) > 0 {
		return fmt.Errorf("validation failed for: %v", failed)
	}
	return nil
}

func selectAppsV2(cfg *config.InfraConfigV2, names []string) ([]config.AppConfigV2, error) {
	sorted, err := config.SortedByDependency(cfg)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return sorted, nil
	}
	wanted := make(map[string]bool, len(names))
	for _, n := range names {
		wanted[n] = true
	}
	var out []config.AppConfigV2
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
