package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	fn()
	require.NoError(t, w.Close())
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func writeV1Fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"apps/caddy", "apps/walls"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0o755))
	}
	yamlBody := `kubeContext: pax
backup:
  remoteHost: pax
  remoteTmp: /tmp/k3s-backups
  localDir: backups
apps:
  - name: caddy
    chart: apps/caddy
    namespace: caddy
    order: 1
    pvcs: [caddy-data]
  - name: walls
    chart: apps/walls
    namespace: walls
    order: 2
    dependsOn: [caddy]
`
	path := filepath.Join(dir, "infra.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yamlBody), 0o644))
	return path
}

func TestV1Compat(t *testing.T) {
	path := writeV1Fixture(t)

	var (
		cfg *InfraConfigV2
		err error
	)
	stderr := captureStderr(t, func() {
		cfg, err = LoadV2(path)
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, APIVersionV2, cfg.APIVersion)
	assert.Equal(t, "pax", cfg.Cluster.Name)
	assert.Equal(t, "pax", cfg.Cluster.KubeContext)
	assert.Equal(t, "k3s", cfg.Cluster.Type)
	assert.Equal(t, "pax", cfg.Backup.RemoteHost)

	require.Len(t, cfg.Apps, 2)
	assert.Equal(t, "caddy", cfg.Apps[0].Name)
	assert.Equal(t, "caddy", cfg.Apps[0].Namespace)
	assert.Equal(t, []string{"caddy-data"}, cfg.Apps[0].PVCs)
	assert.Equal(t, []string{"caddy"}, cfg.Apps[1].DependsOn)

	assert.Contains(t, stderr, "DEPRECATED: missing apiVersion")
	assert.Contains(t, stderr, "'order' field is deprecated")
	assert.Contains(t, stderr, "caddy")
	assert.Contains(t, stderr, "walls")
}

func TestV2Load(t *testing.T) {
	var (
		cfg *InfraConfigV2
		err error
	)
	stderr := captureStderr(t, func() {
		cfg, err = LoadV2("testdata/v2_minimal.yaml")
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, APIVersionV2, cfg.APIVersion)
	assert.Equal(t, "pax", cfg.Cluster.Name)
	assert.Equal(t, "k3s", cfg.Cluster.Type)
	assert.Equal(t, "pax", cfg.Cluster.KubeContext)
	require.Len(t, cfg.Apps, 2)
	assert.Equal(t, "caddy", cfg.Apps[0].Namespace, "namespace should default to name")
	assert.Equal(t, "walls", cfg.Apps[1].Namespace)
	assert.Empty(t, stderr, "v2 config should produce no deprecation warnings")
}

func TestV2CycleDetection(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"apps/caddy", "apps/walls"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0o755))
	}
	yamlBody := `apiVersion: easyinfra/v2
cluster:
  name: test
  type: k3s
  kubeContext: test
backup:
  remoteHost: test
  remoteTmp: /tmp
  localDir: backups
apps:
  - name: caddy
    chart: apps/caddy
    dependsOn: [walls]
  - name: walls
    chart: apps/walls
    dependsOn: [caddy]
`
	path := filepath.Join(dir, "infra.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yamlBody), 0o644))

	_, err := LoadV2(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}
