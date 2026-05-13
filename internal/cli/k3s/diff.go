package k3s

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/diff"
	"github.com/guneet-xyz/easyinfra/pkg/paths"
	"github.com/spf13/cobra"
)

type diffFlags struct {
	app             string
	all             bool
	allowNoCluster  bool
}

func newDiffCmd(flags *RootFlags) *cobra.Command {
	df := &diffFlags{}
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show the diff between the deployed release and a proposed upgrade",
		Long: "Runs `helm diff upgrade` against each selected app to preview the\n" +
			"changes that would be applied. Requires the helm-diff plugin\n" +
			"(https://github.com/databus23/helm-diff). When --allow-no-cluster\n" +
			"is set and the plugin is missing, the command prints a warning\n" +
			"and exits 0 so it remains usable in offline/CI environments.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd, flags, df)
		},
	}
	cmd.Flags().StringVar(&df.app, "app", "", "Diff only this app")
	cmd.Flags().BoolVar(&df.all, "all", false, "Diff all apps in the config")
	cmd.Flags().BoolVar(&df.allowNoCluster, "allow-no-cluster", false,
		"If the helm-diff plugin is missing, print a warning and exit 0 instead of failing")
	return cmd
}

func runDiff(cmd *cobra.Command, flags *RootFlags, df *diffFlags) error {
	if !df.all && df.app == "" {
		return errors.New("specify --app <name> or --all")
	}
	if df.all && df.app != "" {
		return errors.New("cannot specify both --app and --all")
	}

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

	targets, err := selectDiffApps(cfg, df)
	if err != nil {
		return err
	}

	runner := newRunner(cmd, flags)
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	for _, app := range targets {
		output, err := diff.Diff(ctx, runner, app, cfg, baseDir)
		if err != nil {
			if diff.IsPluginMissing(err) && df.allowNoCluster {
				_, _ = fmt.Fprintln(errOut,
					"WARNING: helm-diff not installed; falling back")
				return nil
			}
			return err
		}
		if df.all {
			_, _ = fmt.Fprintf(out, "=== %s ===\n", app.Name)
		}
		_, _ = fmt.Fprintln(out, output)
	}
	return nil
}

func selectDiffApps(cfg *config.InfraConfigV2, df *diffFlags) ([]config.AppConfigV2, error) {
	if df.all {
		return cfg.Apps, nil
	}
	for _, a := range cfg.Apps {
		if a.Name == df.app {
			return []config.AppConfigV2{a}, nil
		}
	}
	return nil, fmt.Errorf("unknown app %q", df.app)
}
