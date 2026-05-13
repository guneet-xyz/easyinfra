// Package render provides offline-capable helm template rendering.
package render

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/helm"
)

// PostRendererMode controls how the post-renderer is handled during render.
type PostRendererMode string

const (
	// PostRendererSkip omits --post-renderer from helm template (safe for CI).
	PostRendererSkip PostRendererMode = "skip"
	// PostRendererRequire passes --post-renderer; errors if binary missing.
	PostRendererRequire PostRendererMode = "require"
	// PostRendererAllowFail passes --post-renderer; falls back without it on failure.
	PostRendererAllowFail PostRendererMode = "allowfail"
)

// Options controls render behavior.
type Options struct {
	BaseDir          string
	PostRendererMode PostRendererMode
}

// Result is the output of rendering a single app.
type Result struct {
	AppName    string
	Manifest   []byte
	Skipped    bool
	SkipReason string
}

// Render renders a single app using helm template.
// Returns a Result with Skipped=true for library charts.
func Render(ctx context.Context, client *helm.Client, app config.AppConfigV2, cfg *config.InfraConfigV2, opts Options) (Result, error) {
	chartPath := filepath.Join(opts.BaseDir, app.Chart)

	isLib, err := client.IsLibraryChart(chartPath)
	if err != nil {
		return Result{}, fmt.Errorf("checking chart type for %s: %w", app.Name, err)
	}
	if isLib {
		return Result{AppName: app.Name, Skipped: true, SkipReason: "library chart"}, nil
	}

	merged := config.MergeAppDefaultsV2(&app, &cfg.Defaults)

	valueFiles := make([]string, len(merged.ValueFiles))
	for i, vf := range merged.ValueFiles {
		valueFiles[i] = filepath.Join(opts.BaseDir, vf)
	}

	mode := opts.PostRendererMode
	if mode == "" {
		mode = PostRendererSkip
	}

	var pr *config.PostRenderer
	if mode != PostRendererSkip {
		pr = merged.PostRenderer
	}

	out, err := client.TemplateWithPostRenderer(ctx, chartPath, valueFiles, pr)
	if err != nil {
		if mode == PostRendererAllowFail {
			out, err = client.TemplateWithPostRenderer(ctx, chartPath, valueFiles, nil)
			if err != nil {
				return Result{}, fmt.Errorf("render %s: %w", app.Name, err)
			}
		} else {
			return Result{}, fmt.Errorf("render %s: %w", app.Name, err)
		}
	}

	return Result{AppName: app.Name, Manifest: []byte(out)}, nil
}

// All renders all apps in cfg, skipping library charts.
func All(ctx context.Context, client *helm.Client, cfg *config.InfraConfigV2, opts Options) ([]Result, error) {
	results := make([]Result, 0, len(cfg.Apps))
	for _, app := range cfg.Apps {
		res, err := Render(ctx, client, app, cfg, opts)
		if err != nil {
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}
