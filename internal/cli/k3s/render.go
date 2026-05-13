package k3s

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/helm"
	"github.com/guneet-xyz/easyinfra/pkg/paths"
	"github.com/guneet-xyz/easyinfra/pkg/render"
	"github.com/spf13/cobra"
)

type renderManifestEntry struct {
	App       string `json:"app"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
}

type renderFlags struct {
	app          string
	all          bool
	noPostRender bool
	outputDir    string
	asJSON       bool
}

func newRenderCmd(flags *RootFlags) *cobra.Command {
	rf := &renderFlags{}
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render charts to YAML manifests (offline-capable)",
		Long: "Render Helm charts to plain Kubernetes YAML manifests using helm template.\n" +
			"By default writes one file per app to the output directory.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRender(cmd, flags, rf)
		},
	}
	cmd.Flags().StringVar(&rf.app, "app", "", "Render only this app (writes to <output-dir>/<app>.yaml)")
	cmd.Flags().BoolVar(&rf.all, "all", false, "Render all apps in the config")
	cmd.Flags().BoolVar(&rf.noPostRender, "no-postrender", false, "Skip post-renderer regardless of config")
	cmd.Flags().StringVar(&rf.outputDir, "output-dir", ".render", "Directory to write rendered manifests to")
	cmd.Flags().BoolVar(&rf.asJSON, "json", false, "emit JSON list of rendered manifests instead of writing files")
	return cmd
}

func runRender(cmd *cobra.Command, flags *RootFlags, rf *renderFlags) error {
	if !rf.all && rf.app == "" {
		return errors.New("specify --app <name> or --all")
	}
	if rf.all && rf.app != "" {
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

	runner := newRunner(cmd, flags)
	client := &helm.Client{Runner: runner, Context: cfg.Cluster.KubeContext}

	opts := render.Options{
		BaseDir:          baseDir,
		PostRendererMode: render.PostRendererMode(cfg.Rendering.PostRendererInCI),
	}
	if rf.noPostRender {
		opts.PostRendererMode = render.PostRendererSkip
	}
	if opts.PostRendererMode == "" {
		opts.PostRendererMode = render.PostRendererSkip
	}

	outputDir := rf.outputDir
	if outputDir == "" {
		outputDir = ".render"
	}

	out := cmd.OutOrStdout()
	ctx := cmd.Context()

	if rf.all {
		results, err := render.All(ctx, client, cfg, opts)
		if err != nil {
			return err
		}

		if rf.asJSON {
			// For JSON output, emit manifest list without writing files
			var entries []renderManifestEntry
			for _, res := range results {
				if !res.Skipped {
					entries = append(entries, renderManifestEntry{
						App:       res.AppName,
						File:      res.AppName + ".yaml",
						SizeBytes: int64(len(res.Manifest)),
					})
				}
			}
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(entries)
		}

		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("creating output dir %s: %w", outputDir, err)
		}
		written := 0
		skipped := 0
		for _, res := range results {
			if res.Skipped {
				_, _ = fmt.Fprintf(out, "SKIP: %s — %s\n", res.AppName, res.SkipReason)
				skipped++
				continue
			}
			path := filepath.Join(outputDir, res.AppName+".yaml")
			if err := os.WriteFile(path, res.Manifest, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", path, err)
			}
			_, _ = fmt.Fprintf(out, "OK:   %s -> %s\n", res.AppName, path)
			written++
		}
		_, _ = fmt.Fprintf(out, "Rendered %d apps to %s (%d skipped)\n", written, outputDir, skipped)
		return nil
	}

	var target *config.AppConfigV2
	for i := range cfg.Apps {
		if cfg.Apps[i].Name == rf.app {
			target = &cfg.Apps[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("unknown app %q", rf.app)
	}

	res, err := render.Render(ctx, client, *target, cfg, opts)
	if err != nil {
		return err
	}

	if rf.asJSON {
		// For JSON output, emit manifest list without writing files
		var entries []renderManifestEntry
		if !res.Skipped {
			entries = append(entries, renderManifestEntry{
				App:       res.AppName,
				File:      res.AppName + ".yaml",
				SizeBytes: int64(len(res.Manifest)),
			})
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	if res.Skipped {
		_, _ = fmt.Fprintf(out, "SKIP: %s — %s\n", res.AppName, res.SkipReason)
		_, _ = fmt.Fprintf(out, "Rendered 0 apps to %s (1 skipped)\n", outputDir)
		return nil
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir %s: %w", outputDir, err)
	}
	path := filepath.Join(outputDir, res.AppName+".yaml")
	if err := os.WriteFile(path, res.Manifest, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	_, _ = fmt.Fprintf(out, "OK:   %s -> %s\n", res.AppName, path)
	_, _ = fmt.Fprintf(out, "Rendered 1 app to %s\n", outputDir)
	return nil
}
