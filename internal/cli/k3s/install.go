package k3s

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/helm"
	"github.com/guneet-xyz/easyinfra/pkg/paths"
	"github.com/spf13/cobra"
)

type opContext struct {
	cfg     *config.InfraConfigV2
	baseDir string
	client  *helm.Client
	apps    []config.AppConfigV2
}

func loadConfig(flags *RootFlags) (*config.InfraConfig, string, error) {
	cfgPath := flags.Config
	if cfgPath == "" {
		p, err := paths.DefaultConfigPath()
		if err != nil {
			return nil, "", fmt.Errorf("resolving config path: %w", err)
		}
		cfgPath = p
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", err
	}
	return cfg, filepath.Dir(cfgPath), nil
}

func loadConfigV2(flags *RootFlags) (*config.InfraConfigV2, string, error) {
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

func selectOpApps(cfg *config.InfraConfig, args []string, all bool) ([]config.AppConfig, error) {
	sorted := config.SortedByOrder(cfg)
	if all {
		if len(args) > 0 {
			return nil, errors.New("cannot specify an app name together with --all")
		}
		return sorted, nil
	}
	if len(args) == 0 {
		return nil, errors.New("specify an app name or use --all")
	}
	target := args[0]
	for _, app := range sorted {
		if app.Name == target {
			return []config.AppConfig{app}, nil
		}
	}
	names := make([]string, 0, len(sorted))
	for _, a := range sorted {
		names = append(names, a.Name)
	}
	return nil, fmt.Errorf("unknown app %q (known: %s)", target, strings.Join(names, ", "))
}

func selectOpAppsV2(cfg *config.InfraConfigV2, args []string, all bool) ([]config.AppConfigV2, error) {
	sorted, err := config.SortedByDependency(cfg)
	if err != nil {
		return nil, err
	}
	if all {
		if len(args) > 0 {
			return nil, errors.New("cannot specify an app name together with --all")
		}
		return sorted, nil
	}
	if len(args) == 0 {
		return nil, errors.New("specify an app name or use --all")
	}
	target := args[0]
	for _, app := range sorted {
		if app.Name == target {
			return []config.AppConfigV2{app}, nil
		}
	}
	names := make([]string, 0, len(sorted))
	for _, a := range sorted {
		names = append(names, a.Name)
	}
	return nil, fmt.Errorf("unknown app %q (known: %s)", target, strings.Join(names, ", "))
}


func prepareOp(cmd *cobra.Command, flags *RootFlags, args []string, all bool) (*opContext, error) {
	cfg, baseDir, err := loadConfigV2(flags)
	if err != nil {
		return nil, err
	}
	apps, err := selectOpAppsV2(cfg, args, all)
	if err != nil {
		return nil, err
	}
	runner := newRunner(cmd, flags)
	if err := config.VerifyKubeContextV2(cmd.Context(), cfg, runner, flags.ConfirmContext); err != nil {
		return nil, err
	}
	return &opContext{
		cfg:     cfg,
		baseDir: baseDir,
		client:  &helm.Client{Runner: runner, Context: cfg.Cluster.KubeContext},
		apps:    apps,
	}, nil
}

func buildInstallOpts(cfg *config.InfraConfigV2, baseDir string, app config.AppConfigV2) helm.InstallOpts {
	valueFiles := config.MergedValueFilesV2(&app, cfg)
	resolved := make([]string, len(valueFiles))
	for i, vf := range valueFiles {
		resolved[i] = filepath.Join(baseDir, vf)
	}
	return helm.InstallOpts{
		Release:      app.Name,
		Chart:        filepath.Join(baseDir, app.Chart),
		Namespace:    app.Namespace,
		ValueFiles:   resolved,
		PostRenderer: config.MergedPostRendererV2(&app, cfg),
	}
}

type installFlags struct {
	all bool
}

func newInstallCmd(rootF *RootFlags) *cobra.Command {
	f := &installFlags{}
	cmd := &cobra.Command{
		Use:   "install [app]",
		Short: "Install one or all apps via helm install",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd, rootF, args, f.all)
		},
	}
	cmd.Flags().BoolVar(&f.all, "all", false, "install all apps in order")
	return cmd
}

func runInstall(cmd *cobra.Command, flags *RootFlags, args []string, all bool) error {
	op, err := prepareOp(cmd, flags, args, all)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	out := cmd.OutOrStdout()
	for _, app := range op.apps {
		opts := buildInstallOpts(op.cfg, op.baseDir, app)
		if err := op.client.Install(ctx, opts); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "Installed %s\n", app.Name)
	}
	return nil
}
