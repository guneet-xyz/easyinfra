package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	execpkg "github.com/guneet-xyz/easyinfra/pkg/exec"
)

const testdataInfra = "../../testdata/infra"

func testdataPath() string {
	return filepath.Join(testdataInfra, "infra.yaml")
}

func TestLoadValid(t *testing.T) {
	cfg, err := Load(testdataPath())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "test-ctx", cfg.KubeContext)
	assert.Len(t, cfg.Apps, 2)
	assert.Equal(t, "alpha", cfg.Apps[0].Name)
	assert.Equal(t, "beta", cfg.Apps[1].Name)
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("nonexistent.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent.yaml")
}

func TestLoadParseError(t *testing.T) {
	tmp := t.TempDir()
	bad := filepath.Join(tmp, "bad.yaml")
	require.NoError(t, os.WriteFile(bad, []byte("kubeContext: [unterminated"), 0o644))
	_, err := Load(bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing")
}

func TestValidateMissingKubeContext(t *testing.T) {
	cfg := &InfraConfig{
		Apps: []AppConfig{{Name: "a", Chart: "charts/alpha", Namespace: "ns", Order: 1}},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubeContext")
}

func TestValidateNoApps(t *testing.T) {
	cfg := &InfraConfig{KubeContext: "ctx"}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one app")
}

func TestValidateDuplicateAppName(t *testing.T) {
	cfg := &InfraConfig{
		KubeContext: "ctx",
		Apps: []AppConfig{
			{Name: "a", Chart: "charts/alpha", Namespace: "ns", Order: 1},
			{Name: "a", Chart: "charts/beta", Namespace: "ns", Order: 2},
		},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestValidateMissingChart(t *testing.T) {
	cfg := &InfraConfig{
		KubeContext: "ctx",
		Apps: []AppConfig{
			{Name: "a", Chart: "", Namespace: "ns", Order: 1},
		},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chart is required")
}

func TestValidateMissingNamespace(t *testing.T) {
	cfg := &InfraConfig{
		KubeContext: "ctx",
		Apps: []AppConfig{
			{Name: "a", Chart: "charts/alpha", Namespace: "", Order: 1},
		},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace is required")
}

func TestValidateMissingOrder(t *testing.T) {
	cfg := &InfraConfig{
		KubeContext: "ctx",
		Apps: []AppConfig{
			{Name: "a", Chart: "charts/alpha", Namespace: "ns", Order: 0},
		},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "order")
}

func TestValidateDependsOnUnknown(t *testing.T) {
	cfg := &InfraConfig{
		KubeContext: "ctx",
		Apps: []AppConfig{
			{Name: "a", Chart: "charts/alpha", Namespace: "ns", Order: 1, DependsOn: []string{"ghost"}},
		},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown app")
}

func TestValidateCycleDetection(t *testing.T) {
	cfg := &InfraConfig{
		KubeContext: "ctx",
		Apps: []AppConfig{
			{Name: "a", Chart: "charts/alpha", Namespace: "ns", Order: 1, DependsOn: []string{"b"}},
			{Name: "b", Chart: "charts/beta", Namespace: "ns", Order: 2, DependsOn: []string{"a"}},
		},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestValidateBackupRequiredWithPVCs(t *testing.T) {
	cfg := &InfraConfig{
		KubeContext: "ctx",
		Apps: []AppConfig{
			{Name: "a", Chart: "charts/alpha", Namespace: "ns", Order: 1, PVCs: []string{"data"}},
		},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remoteHost")
}

func TestValidateMissingChartPath(t *testing.T) {
	cfg := &InfraConfig{
		KubeContext: "ctx",
		Apps: []AppConfig{
			{Name: "a", Chart: "charts/missing", Namespace: "ns", Order: 1},
		},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chart path")
}

func TestValidateMissingValueFile(t *testing.T) {
	cfg := &InfraConfig{
		KubeContext: "ctx",
		Apps: []AppConfig{
			{Name: "a", Chart: "charts/alpha", Namespace: "ns", Order: 1, ValueFiles: []string{"missing.yaml"}},
		},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valueFile")
}

func TestValidateMissingDefaultsValueFile(t *testing.T) {
	cfg := &InfraConfig{
		KubeContext: "ctx",
		Defaults:    Defaults{ValueFiles: []string{"missing.yaml"}},
		Apps: []AppConfig{
			{Name: "a", Chart: "charts/alpha", Namespace: "ns", Order: 1},
		},
	}
	err := Validate(cfg, testdataInfra)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "defaults.valueFiles")
}

func TestMergedPostRendererDefaults(t *testing.T) {
	def := &PostRenderer{Command: "default", Args: []string{"x"}}
	cfg := &InfraConfig{Defaults: Defaults{PostRenderer: def}}
	app := &AppConfig{}
	got := MergedPostRenderer(app, cfg)
	assert.Equal(t, def, got)
}

func TestMergedPostRendererAppOverride(t *testing.T) {
	def := &PostRenderer{Command: "default"}
	own := &PostRenderer{Command: "own"}
	cfg := &InfraConfig{Defaults: Defaults{PostRenderer: def}}
	app := &AppConfig{PostRenderer: own}
	got := MergedPostRenderer(app, cfg)
	assert.Equal(t, own, got)
}

func TestMergedValueFiles(t *testing.T) {
	cfg := &InfraConfig{Defaults: Defaults{ValueFiles: []string{"a.yaml"}}}
	app := &AppConfig{ValueFiles: []string{"b.yaml"}}
	got := MergedValueFiles(app, cfg)
	assert.Equal(t, []string{"a.yaml", "b.yaml"}, got)
}

func TestMergedValueFilesDedup(t *testing.T) {
	cfg := &InfraConfig{Defaults: Defaults{ValueFiles: []string{"a.yaml", "shared.yaml"}}}
	app := &AppConfig{ValueFiles: []string{"shared.yaml", "b.yaml"}}
	got := MergedValueFiles(app, cfg)
	assert.Equal(t, []string{"a.yaml", "shared.yaml", "b.yaml"}, got)
}

func TestSortedByOrder(t *testing.T) {
	cfg := &InfraConfig{
		Apps: []AppConfig{
			{Name: "c", Order: 3},
			{Name: "a", Order: 1},
			{Name: "b", Order: 2},
		},
	}
	sorted := SortedByOrder(cfg)
	assert.Equal(t, "a", sorted[0].Name)
	assert.Equal(t, "b", sorted[1].Name)
	assert.Equal(t, "c", sorted[2].Name)
}

func TestSortedByOrderStable(t *testing.T) {
	cfg := &InfraConfig{
		Apps: []AppConfig{
			{Name: "first", Order: 1},
			{Name: "second", Order: 1},
			{Name: "third", Order: 1},
		},
	}
	sorted := SortedByOrder(cfg)
	assert.Equal(t, "first", sorted[0].Name)
	assert.Equal(t, "second", sorted[1].Name)
	assert.Equal(t, "third", sorted[2].Name)
}

func TestVerifyKubeContextMatch(t *testing.T) {
	runner := &execpkg.FakeRunner{
		Default: execpkg.FakeResponse{Stdout: "test-ctx"},
	}
	cfg := &InfraConfig{KubeContext: "test-ctx"}
	err := VerifyKubeContext(context.Background(), cfg, runner, false)
	assert.NoError(t, err)
}

func TestVerifyKubeContextMismatch(t *testing.T) {
	runner := &execpkg.FakeRunner{
		Default: execpkg.FakeResponse{Stdout: "other-ctx"},
	}
	cfg := &InfraConfig{KubeContext: "test-ctx"}
	err := VerifyKubeContext(context.Background(), cfg, runner, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test-ctx")
	assert.Contains(t, err.Error(), "other-ctx")
}

func TestVerifyKubeContextForce(t *testing.T) {
	runner := &execpkg.FakeRunner{
		Default: execpkg.FakeResponse{Stdout: "other-ctx"},
	}
	cfg := &InfraConfig{KubeContext: "test-ctx"}
	err := VerifyKubeContext(context.Background(), cfg, runner, true)
	assert.NoError(t, err)
}

func TestVerifyKubeContextRunnerError(t *testing.T) {
	runner := &execpkg.FakeRunner{
		Default: execpkg.FakeResponse{Err: assert.AnError},
	}
	cfg := &InfraConfig{KubeContext: "test-ctx"}
	err := VerifyKubeContext(context.Background(), cfg, runner, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubectl context")
}
