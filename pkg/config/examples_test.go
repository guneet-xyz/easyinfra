package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestExamplesValid(t *testing.T) {
	t.Run("v1_example", func(t *testing.T) {
		data, err := os.ReadFile("example_infra.yaml")
		require.NoError(t, err)
		var cfg InfraConfig
		require.NoError(t, yaml.Unmarshal(data, &cfg))
		assert.Equal(t, "pax", cfg.KubeContext)
		assert.Len(t, cfg.Apps, 11)
	})

	t.Run("v2_example", func(t *testing.T) {
		data, err := os.ReadFile("example_infra_v2.yaml")
		require.NoError(t, err)
		var cfg InfraConfigV2
		require.NoError(t, yaml.Unmarshal(data, &cfg))
		assert.Equal(t, "easyinfra/v2", cfg.APIVersion)
		assert.Equal(t, "pax", cfg.Cluster.Name)
		assert.Len(t, cfg.Apps, 11)
		assert.Len(t, cfg.Phases, 2)
		assert.Equal(t, "core", cfg.Phases[0].Name)
		assert.Equal(t, "workloads", cfg.Phases[1].Name)
		assert.Equal(t, "skip", cfg.Rendering.PostRendererInCI)
		assert.Equal(t, 14, cfg.Backup.Retention.Keep)
	})

	t.Run("v2_full_fixture", func(t *testing.T) {
		data, err := os.ReadFile("testdata/v2_full.yaml")
		require.NoError(t, err)
		var cfg InfraConfigV2
		require.NoError(t, yaml.Unmarshal(data, &cfg))
		assert.Equal(t, "easyinfra/v2", cfg.APIVersion)
		assert.Len(t, cfg.Apps, 2)
		assert.Equal(t, "720h", cfg.Backup.Retention.OlderThan)
	})
}
