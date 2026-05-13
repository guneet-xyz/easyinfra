package config

import (
	"context"
	"testing"

	execpkg "github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyKubeContextV2Match(t *testing.T) {
	runner := &execpkg.FakeRunner{Default: execpkg.FakeResponse{Stdout: "ctx-a\n"}}
	cfg := &InfraConfigV2{Cluster: ClusterConfig{KubeContext: "ctx-a"}}
	require.NoError(t, VerifyKubeContextV2(context.Background(), cfg, runner, false))
}

func TestVerifyKubeContextV2Mismatch(t *testing.T) {
	runner := &execpkg.FakeRunner{Default: execpkg.FakeResponse{Stdout: "ctx-b\n"}}
	cfg := &InfraConfigV2{Cluster: ClusterConfig{KubeContext: "ctx-a"}}
	err := VerifyKubeContextV2(context.Background(), cfg, runner, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

func TestVerifyKubeContextV2ForceAllowsMismatch(t *testing.T) {
	runner := &execpkg.FakeRunner{Default: execpkg.FakeResponse{Stdout: "ctx-b\n"}}
	cfg := &InfraConfigV2{Cluster: ClusterConfig{KubeContext: "ctx-a"}}
	require.NoError(t, VerifyKubeContextV2(context.Background(), cfg, runner, true))
}

func TestVerifyKubeContextV2RunnerError(t *testing.T) {
	runner := &execpkg.FakeRunner{Default: execpkg.FakeResponse{Err: assert.AnError}}
	cfg := &InfraConfigV2{Cluster: ClusterConfig{KubeContext: "ctx-a"}}
	err := VerifyKubeContextV2(context.Background(), cfg, runner, false)
	require.Error(t, err)
}

func TestMergedPostRendererV2Defaults(t *testing.T) {
	def := &PostRenderer{Command: "default"}
	cfg := &InfraConfigV2{Defaults: Defaults{PostRenderer: def}}
	app := &AppConfigV2{}
	assert.Equal(t, def, MergedPostRendererV2(app, cfg))
}

func TestMergedPostRendererV2AppOverride(t *testing.T) {
	def := &PostRenderer{Command: "default"}
	own := &PostRenderer{Command: "own"}
	cfg := &InfraConfigV2{Defaults: Defaults{PostRenderer: def}}
	app := &AppConfigV2{PostRenderer: own}
	assert.Equal(t, own, MergedPostRendererV2(app, cfg))
}

func TestMergedValueFilesV2(t *testing.T) {
	cfg := &InfraConfigV2{Defaults: Defaults{ValueFiles: []string{"a.yaml"}}}
	app := &AppConfigV2{ValueFiles: []string{"b.yaml"}}
	assert.Equal(t, []string{"a.yaml", "b.yaml"}, MergedValueFilesV2(app, cfg))
}

func TestMergedValueFilesV2Dedup(t *testing.T) {
	cfg := &InfraConfigV2{Defaults: Defaults{ValueFiles: []string{"a.yaml", "shared.yaml"}}}
	app := &AppConfigV2{ValueFiles: []string{"shared.yaml", "b.yaml"}}
	assert.Equal(t, []string{"a.yaml", "shared.yaml", "b.yaml"}, MergedValueFilesV2(app, cfg))
}

func TestLoadV1(t *testing.T) {
	cfg, err := LoadV1("../../testdata/infra/infra.yaml")
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "test-ctx", cfg.KubeContext)
	require.NotEmpty(t, cfg.Apps)
}

func TestMigrateV1ToV2ForOrdering(t *testing.T) {
	v1 := &InfraConfig{
		KubeContext: "ctx-a",
		Apps: []AppConfig{
			{Name: "a", Chart: "apps/a", Namespace: "a", Order: 1},
			{Name: "b", Chart: "apps/b", Namespace: "b", Order: 2, DependsOn: []string{"a"}},
		},
		Backup: BackupConfig{RemoteHost: "host", LocalDir: "backups"},
	}
	v2 := MigrateV1ToV2ForOrdering(v1)
	require.NotNil(t, v2)
	assert.Equal(t, APIVersionV2, v2.APIVersion)
	assert.Equal(t, "ctx-a", v2.Cluster.KubeContext)
	assert.Equal(t, "k3s", v2.Cluster.Type)
	require.Len(t, v2.Apps, 2)
	assert.Equal(t, "a", v2.Apps[0].Name)
	assert.Equal(t, []string{"a"}, v2.Apps[1].DependsOn)
	assert.Equal(t, "host", v2.Backup.RemoteHost)
}

func TestSortedByDependency(t *testing.T) {
	cfg := &InfraConfigV2{
		Apps: []AppConfigV2{
			{Name: "b", DependsOn: []string{"a"}, Order: 2},
			{Name: "a", Order: 1},
			{Name: "c", DependsOn: []string{"b"}, Order: 3},
		},
	}
	sorted, err := SortedByDependency(cfg)
	require.NoError(t, err)
	require.Len(t, sorted, 3)
	assert.Equal(t, "a", sorted[0].Name)
	assert.Equal(t, "b", sorted[1].Name)
	assert.Equal(t, "c", sorted[2].Name)
}

func TestSortedByDependencyEmpty(t *testing.T) {
	got, err := SortedByDependency(nil)
	require.NoError(t, err)
	assert.Empty(t, got)

	got, err = SortedByDependency(&InfraConfigV2{})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestSortedByDependencyTieBreakOrder(t *testing.T) {
	cfg := &InfraConfigV2{
		Apps: []AppConfigV2{
			{Name: "z", Order: 1},
			{Name: "y", Order: 2},
			{Name: "x", Order: 3},
		},
	}
	sorted, err := SortedByDependency(cfg)
	require.NoError(t, err)
	require.Len(t, sorted, 3)
	assert.Equal(t, "z", sorted[0].Name)
	assert.Equal(t, "y", sorted[1].Name)
	assert.Equal(t, "x", sorted[2].Name)
}

func TestSortedByDependencyPhases(t *testing.T) {
	cfg := &InfraConfigV2{
		Phases: []PhaseConfig{
			{Name: "core", Apps: []string{"a"}},
			{Name: "workloads", Apps: []string{"b"}},
		},
		Apps: []AppConfigV2{
			{Name: "b", Order: 1},
			{Name: "a", Order: 2},
		},
	}
	sorted, err := SortedByDependency(cfg)
	require.NoError(t, err)
	require.Len(t, sorted, 2)
	assert.Equal(t, "a", sorted[0].Name)
	assert.Equal(t, "b", sorted[1].Name)
}

func TestSortedByDependencyCycle(t *testing.T) {
	cfg := &InfraConfigV2{
		Apps: []AppConfigV2{
			{Name: "a", DependsOn: []string{"b"}},
			{Name: "b", DependsOn: []string{"a"}},
		},
	}
	_, err := SortedByDependency(cfg)
	require.Error(t, err)
}
