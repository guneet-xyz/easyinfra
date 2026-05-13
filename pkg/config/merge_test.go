package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeValueFilesOrder(t *testing.T) {
	defaults := &Defaults{ValueFiles: []string{"shared.yaml"}}
	app := &AppConfigV2{Name: "walls", Chart: "apps/walls", ValueFiles: []string{"walls.yaml"}}
	result := MergeAppDefaultsV2(app, defaults)
	assert.Equal(t, []string{"shared.yaml", "walls.yaml"}, result.ValueFiles,
		"shared values must come before app-specific values")
}

func TestMergeValueFilesDedup(t *testing.T) {
	defaults := &Defaults{ValueFiles: []string{"shared.yaml", "common.yaml"}}
	app := &AppConfigV2{Name: "walls", Chart: "apps/walls", ValueFiles: []string{"shared.yaml", "walls.yaml"}}
	result := MergeAppDefaultsV2(app, defaults)
	assert.Equal(t, []string{"shared.yaml", "common.yaml", "walls.yaml"}, result.ValueFiles,
		"shared.yaml must not appear twice")
}

func TestMergePostRendererOverride(t *testing.T) {
	defaults := &Defaults{PostRenderer: &PostRenderer{Command: "obscuro", Args: []string{"inject"}}}
	app := &AppConfigV2{Name: "walls", Chart: "apps/walls"}
	result := MergeAppDefaultsV2(app, defaults)
	require.NotNil(t, result.PostRenderer)
	assert.Equal(t, "obscuro", result.PostRenderer.Command)
	assert.Equal(t, []string{"inject"}, result.PostRenderer.Args)
}

func TestMergePostRendererAppOverridesDefault(t *testing.T) {
	defaults := &Defaults{PostRenderer: &PostRenderer{Command: "obscuro", Args: []string{"inject"}}}
	appPR := &PostRenderer{Command: "custom", Args: []string{"run"}}
	app := &AppConfigV2{Name: "walls", Chart: "apps/walls", PostRenderer: appPR}
	result := MergeAppDefaultsV2(app, defaults)
	require.NotNil(t, result.PostRenderer)
	assert.Equal(t, "custom", result.PostRenderer.Command, "app-level postrenderer must override defaults")
}

func TestMergeNamespaceDefault(t *testing.T) {
	defaults := &Defaults{}
	app := &AppConfigV2{Name: "walls", Chart: "apps/walls"}
	result := MergeAppDefaultsV2(app, defaults)
	assert.Equal(t, "walls", result.Namespace, "namespace must default to app name")
}

func TestMergeAppNoDefaults(t *testing.T) {
	defaults := &Defaults{}
	app := &AppConfigV2{Name: "walls", Chart: "apps/walls", ValueFiles: []string{"walls.yaml"}}
	result := MergeAppDefaultsV2(app, defaults)
	assert.Equal(t, []string{"walls.yaml"}, result.ValueFiles)
	assert.Nil(t, result.PostRenderer)
}

func TestMergeDefaultsOnly(t *testing.T) {
	defaults := &Defaults{ValueFiles: []string{"shared.yaml"}}
	app := &AppConfigV2{Name: "walls", Chart: "apps/walls"}
	result := MergeAppDefaultsV2(app, defaults)
	assert.Equal(t, []string{"shared.yaml"}, result.ValueFiles)
}

func TestMergeIncludesConflict(t *testing.T) {
	base := &InfraConfigV2{
		Apps: []AppConfigV2{{Name: "caddy", Chart: "apps/caddy"}},
	}
	partials := []PartialConfig{
		{Source: "includes/extra.yaml", Apps: []AppConfigV2{{Name: "caddy", Chart: "apps/caddy2"}}},
	}
	_, err := MergeIncludesV2(base, partials)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate app")
	assert.Contains(t, err.Error(), "caddy")
}

func TestMergeIncludesAppend(t *testing.T) {
	base := &InfraConfigV2{
		Apps: []AppConfigV2{{Name: "caddy", Chart: "apps/caddy"}},
	}
	partials := []PartialConfig{
		{Source: "includes/extra.yaml", Apps: []AppConfigV2{{Name: "walls", Chart: "apps/walls"}}},
	}
	result, err := MergeIncludesV2(base, partials)
	require.NoError(t, err)
	assert.Len(t, result.Apps, 2)
	assert.Equal(t, "caddy", result.Apps[0].Name)
	assert.Equal(t, "walls", result.Apps[1].Name)
}

func TestMergeIncludesValueFilesAppended(t *testing.T) {
	base := &InfraConfigV2{
		Defaults: Defaults{ValueFiles: []string{"shared.yaml"}},
	}
	partials := []PartialConfig{
		{Source: "includes/extra.yaml", Defaults: &Defaults{ValueFiles: []string{"extra.yaml"}}},
	}
	result, err := MergeIncludesV2(base, partials)
	require.NoError(t, err)
	assert.Equal(t, []string{"shared.yaml", "extra.yaml"}, result.Defaults.ValueFiles)
}
