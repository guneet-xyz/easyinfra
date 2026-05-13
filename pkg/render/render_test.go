package render

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/guneet-xyz/easyinfra/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	calls    [][]string
	response string
	errOn    func(args []string) bool
}

func (f *fakeRunner) Run(_ context.Context, cmd string, args ...string) (string, string, error) {
	all := append([]string{cmd}, args...)
	f.calls = append(f.calls, all)
	if f.errOn != nil && f.errOn(all) {
		return "", "binary not found", fmt.Errorf("exit status 1")
	}
	return f.response, "", nil
}

func (f *fakeRunner) RunInteractive(_ context.Context, _ string, _ ...string) error {
	return nil
}

var _ exec.Runner = (*fakeRunner)(nil)

func writeChart(t *testing.T, baseDir, chartRel, chartYaml string) {
	t.Helper()
	full := filepath.Join(baseDir, chartRel)
	require.NoError(t, os.MkdirAll(full, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(full, "Chart.yaml"), []byte(chartYaml), 0o644))
}

func makeTestCfg() *config.InfraConfigV2 {
	return &config.InfraConfigV2{
		Defaults: config.Defaults{
			PostRenderer: &config.PostRenderer{Command: "obscuro", Args: []string{"inject"}},
			ValueFiles:   []string{"values-shared.yaml"},
		},
		Apps: []config.AppConfigV2{
			{Name: "walls", Chart: "apps/walls"},
		},
	}
}

func setupApp(t *testing.T) (string, *config.InfraConfigV2) {
	t.Helper()
	baseDir := t.TempDir()
	writeChart(t, baseDir, "apps/walls", "apiVersion: v2\nname: walls\nversion: 0.1.0\n")
	return baseDir, makeTestCfg()
}

func hasArg(calls [][]string, target string) bool {
	for _, call := range calls {
		for _, arg := range call {
			if arg == target {
				return true
			}
		}
	}
	return false
}

func TestRenderSkipPostRenderer(t *testing.T) {
	baseDir, cfg := setupApp(t)
	runner := &fakeRunner{response: "kind: Deployment\nmetadata:\n  name: walls\n"}
	client := &helm.Client{Runner: runner}
	opts := Options{BaseDir: baseDir, PostRendererMode: PostRendererSkip}

	res, err := Render(context.Background(), client, cfg.Apps[0], cfg, opts)
	require.NoError(t, err)
	assert.False(t, res.Skipped)
	assert.Contains(t, string(res.Manifest), "walls")
	assert.False(t, hasArg(runner.calls, "--post-renderer"), "skip mode must not pass --post-renderer")
}

func TestRenderRequirePostRenderer(t *testing.T) {
	baseDir, cfg := setupApp(t)
	runner := &fakeRunner{response: "kind: Deployment\nmetadata:\n  name: walls\n"}
	client := &helm.Client{Runner: runner}
	opts := Options{BaseDir: baseDir, PostRendererMode: PostRendererRequire}

	res, err := Render(context.Background(), client, cfg.Apps[0], cfg, opts)
	require.NoError(t, err)
	assert.False(t, res.Skipped)
	assert.True(t, hasArg(runner.calls, "--post-renderer"), "require mode must pass --post-renderer")
}

func TestRenderRequireFailsOnError(t *testing.T) {
	baseDir, cfg := setupApp(t)
	runner := &fakeRunner{
		response: "kind: Deployment\n",
		errOn: func(args []string) bool {
			for _, a := range args {
				if a == "--post-renderer" {
					return true
				}
			}
			return false
		},
	}
	client := &helm.Client{Runner: runner}
	opts := Options{BaseDir: baseDir, PostRendererMode: PostRendererRequire}

	_, err := Render(context.Background(), client, cfg.Apps[0], cfg, opts)
	require.Error(t, err, "require mode must surface post-renderer errors")
}

func TestRenderAllowFailFallsBack(t *testing.T) {
	baseDir, cfg := setupApp(t)
	runner := &fakeRunner{
		response: "kind: Deployment\nmetadata:\n  name: walls\n",
		errOn: func(args []string) bool {
			for _, a := range args {
				if a == "--post-renderer" {
					return true
				}
			}
			return false
		},
	}
	client := &helm.Client{Runner: runner}
	opts := Options{BaseDir: baseDir, PostRendererMode: PostRendererAllowFail}

	res, err := Render(context.Background(), client, cfg.Apps[0], cfg, opts)
	require.NoError(t, err, "allowfail mode must not error when post-renderer fails")
	assert.False(t, res.Skipped)
	assert.Contains(t, string(res.Manifest), "walls")
	assert.Len(t, runner.calls, 2, "allowfail must retry once without post-renderer")
}

func TestRenderSkipsLibraryChart(t *testing.T) {
	baseDir := t.TempDir()
	writeChart(t, baseDir, "apps/lib", "apiVersion: v2\nname: lib\ntype: library\nversion: 0.1.0\n")
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{{Name: "lib", Chart: "apps/lib"}},
	}
	runner := &fakeRunner{response: "should not be called"}
	client := &helm.Client{Runner: runner}

	res, err := Render(context.Background(), client, cfg.Apps[0], cfg, Options{BaseDir: baseDir})
	require.NoError(t, err)
	assert.True(t, res.Skipped)
	assert.Equal(t, "library chart", res.SkipReason)
	assert.Empty(t, runner.calls, "library chart must not invoke helm")
}

func TestRenderAll(t *testing.T) {
	baseDir := t.TempDir()
	writeChart(t, baseDir, "apps/walls", "apiVersion: v2\nname: walls\nversion: 0.1.0\n")
	writeChart(t, baseDir, "apps/lib", "apiVersion: v2\nname: lib\ntype: library\nversion: 0.1.0\n")
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{
			{Name: "walls", Chart: "apps/walls"},
			{Name: "lib", Chart: "apps/lib"},
		},
	}
	runner := &fakeRunner{response: "kind: Deployment\n"}
	client := &helm.Client{Runner: runner}

	results, err := RenderAll(context.Background(), client, cfg, Options{BaseDir: baseDir})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.False(t, results[0].Skipped)
	assert.True(t, results[1].Skipped)
}
