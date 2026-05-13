package k3s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/deps"
	"github.com/guneet-xyz/easyinfra/pkg/paths"
	"github.com/spf13/cobra"
)

type depsFlags struct {
	app  string
	all  bool
	json bool
}

func newDepsCmd(flags *RootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps",
		Short: "Manage helm chart dependencies",
	}
	cmd.AddCommand(newDepsCheckCmd(flags), newDepsUpdateCmd(flags))
	return cmd
}

func newDepsCheckCmd(flags *RootFlags) *cobra.Command {
	df := &depsFlags{}
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check helm chart dependencies (Chart.lock freshness, file:// deps)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDepsCheck(cmd, flags, df)
		},
	}
	cmd.Flags().StringVar(&df.app, "app", "", "Check only this app")
	cmd.Flags().BoolVar(&df.all, "all", false, "Check all apps in the config")
	cmd.Flags().BoolVar(&df.json, "json", false, "Emit JSON output")
	return cmd
}

func newDepsUpdateCmd(flags *RootFlags) *cobra.Command {
	df := &depsFlags{}
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Run `helm dependency update` for chart(s)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDepsUpdate(cmd, flags, df)
		},
	}
	cmd.Flags().StringVar(&df.app, "app", "", "Update only this app")
	cmd.Flags().BoolVar(&df.all, "all", false, "Update all apps in the config")
	return cmd
}

func loadDepsConfig(flags *RootFlags) (*config.InfraConfigV2, string, error) {
	cfgPath := flags.Config
	if cfgPath == "" {
		p, err := paths.DefaultConfigPath()
		if err != nil {
			return nil, "", fmt.Errorf("resolving config path: %w", err)
		}
		cfgPath = p
	}
	cfg, err := config.LoadV2(cfgPath)
	if err != nil {
		return nil, "", err
	}
	return cfg, filepath.Dir(cfgPath), nil
}

func selectDepsApps(cfg *config.InfraConfigV2, df *depsFlags) ([]config.AppConfigV2, error) {
	if df.all && df.app != "" {
		return nil, errors.New("cannot specify both --app and --all")
	}
	if df.app == "" {
		return cfg.Apps, nil
	}
	for i := range cfg.Apps {
		if cfg.Apps[i].Name == df.app {
			return []config.AppConfigV2{cfg.Apps[i]}, nil
		}
	}
	return nil, fmt.Errorf("unknown app %q", df.app)
}

type depsCheckReport struct {
	Apps   []appCheckResult `json:"apps"`
	Failed bool             `json:"failed"`
}

type appCheckResult struct {
	App    string       `json:"app"`
	Issues []deps.Issue `json:"issues"`
	Error  string       `json:"error,omitempty"`
}

func runDepsCheck(cmd *cobra.Command, flags *RootFlags, df *depsFlags) error {
	cfg, baseDir, err := loadDepsConfig(flags)
	if err != nil {
		return err
	}
	apps, err := selectDepsApps(cfg, df)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	report := depsCheckReport{}
	for _, app := range apps {
		chartDir := filepath.Join(baseDir, app.Chart)
		issues, cerr := deps.Check(ctx, chartDir)
		res := appCheckResult{App: app.Name, Issues: issues}
		if cerr != nil {
			res.Error = cerr.Error()
			report.Failed = true
		}
		if len(issues) > 0 {
			report.Failed = true
		}
		report.Apps = append(report.Apps, res)
	}

	out := cmd.OutOrStdout()
	if df.json {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		writeDepsCheckText(out, report)
	}

	if report.Failed {
		return fmt.Errorf("dependency issues detected")
	}
	return nil
}

func writeDepsCheckText(w io.Writer, report depsCheckReport) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "APP\tKIND\tMESSAGE")
	total := 0
	for _, app := range report.Apps {
		if app.Error != "" {
			_, _ = fmt.Fprintf(tw, "%s\terror\t%s\n", app.App, app.Error)
			total++
			continue
		}
		if len(app.Issues) == 0 {
			_, _ = fmt.Fprintf(tw, "%s\tok\t-\n", app.App)
			continue
		}
		for _, iss := range app.Issues {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", app.App, iss.Kind, iss.Message)
			total++
		}
	}
	_ = tw.Flush()
	if report.Failed {
		_, _ = fmt.Fprintf(w, "\n%d issue(s) found\n", total)
	} else {
		_, _ = fmt.Fprintln(w, "\nAll dependencies OK")
	}
}

func runDepsUpdate(cmd *cobra.Command, flags *RootFlags, df *depsFlags) error {
	cfg, baseDir, err := loadDepsConfig(flags)
	if err != nil {
		return err
	}
	apps, err := selectDepsApps(cfg, df)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	runner := newRunner(cmd, flags)
	out := cmd.OutOrStdout()

	var failed []string
	for _, app := range apps {
		chartDir := filepath.Join(baseDir, app.Chart)
		if err := deps.Update(ctx, runner, chartDir); err != nil {
			_, _ = fmt.Fprintf(out, "FAIL: %s — %v\n", app.Name, err)
			failed = append(failed, app.Name)
			continue
		}
		_, _ = fmt.Fprintf(out, "OK:   %s\n", app.Name)
	}

	if len(failed) > 0 {
		return fmt.Errorf("dependency update failed for %d app(s): %v", len(failed), failed)
	}
	return nil
}
