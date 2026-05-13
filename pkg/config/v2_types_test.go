package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnmarshalV2(t *testing.T) {
	data, err := os.ReadFile("testdata/v2_minimal.yaml")
	require.NoError(t, err)
	var cfg InfraConfigV2
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "pax", cfg.Cluster.Name)
	assert.Equal(t, "k3s", cfg.Cluster.Type)
	assert.Equal(t, "pax", cfg.Cluster.KubeContext)
	assert.Equal(t, "easyinfra/v2", cfg.APIVersion)
	assert.Len(t, cfg.Apps, 2)
	assert.Equal(t, "caddy", cfg.Apps[0].Name)
	assert.Equal(t, "walls", cfg.Apps[1].Name)
	assert.Equal(t, []string{"caddy"}, cfg.Apps[1].DependsOn)
}

func TestDetectAPIVersion(t *testing.T) {
	v2data := []byte("apiVersion: easyinfra/v2\ncluster:\n  name: pax\n")
	assert.Equal(t, APIVersionV2, DetectAPIVersion(v2data))

	v1data := []byte("kubeContext: pax\napps: []\n")
	assert.Equal(t, APIVersionV1, DetectAPIVersion(v1data))
}

func TestV1StillCompiles(t *testing.T) {
	var cfg InfraConfig
	cfg.KubeContext = "pax"
	cfg.Apps = []AppConfig{{Name: "x", Chart: "y", Order: 1}}
	assert.Equal(t, "pax", cfg.KubeContext)
	assert.Equal(t, 1, cfg.Apps[0].Order)
}
