// Package diff provides helm-diff plugin integration for previewing
// changes between the deployed release and a proposed upgrade.
package diff

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

// ErrPluginMissing indicates the helm-diff plugin is not installed.
var ErrPluginMissing = errors.New("helm-diff plugin not installed")

// Diff shells out to `helm diff upgrade` for the given app and returns
// the plugin's stdout. When the helm-diff plugin is not installed the
// returned error wraps ErrPluginMissing (use IsPluginMissing to detect).
func Diff(ctx context.Context, runner exec.Runner, app config.AppConfigV2, cfg *config.InfraConfigV2, baseDir string) (string, error) {
	if runner == nil {
		return "", errors.New("diff: runner is nil")
	}
	if cfg == nil {
		return "", errors.New("diff: cfg is nil")
	}

	merged := config.MergeAppDefaultsV2(&app, &cfg.Defaults)

	args := []string{"diff", "upgrade", merged.Name, filepath.Join(baseDir, merged.Chart)}
	if merged.Namespace != "" {
		args = append(args, "-n", merged.Namespace)
	}
	for _, vf := range merged.ValueFiles {
		args = append(args, "-f", filepath.Join(baseDir, vf))
	}
	if pr := merged.PostRenderer; pr != nil && pr.Command != "" {
		args = append(args, "--post-renderer", pr.Command)
		for _, a := range pr.Args {
			args = append(args, "--post-renderer-args", a)
		}
	}

	stdout, stderr, err := runner.Run(ctx, "helm", args...)
	if err != nil {
		if IsPluginMissing(errors.New(stderr)) {
			return "", fmt.Errorf("%w: %s", ErrPluginMissing, strings.TrimSpace(stderr))
		}
		return "", fmt.Errorf("helm diff upgrade %s: %w\n%s", merged.Name, err, stderr)
	}
	return stdout, nil
}

// IsPluginMissing reports whether err indicates the helm-diff plugin
// is not installed. It checks both the wrapped ErrPluginMissing and
// well-known stderr substrings emitted by helm.
func IsPluginMissing(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrPluginMissing) {
		return true
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, `unknown command "diff"`):
		return true
	case strings.Contains(msg, `plugin "diff" not found`):
		return true
	case strings.Contains(msg, `plugin not found`) && strings.Contains(msg, "diff"):
		return true
	}
	return false
}
