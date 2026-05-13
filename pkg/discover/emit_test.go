package discover

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/guneet-xyz/easyinfra/pkg/config"
)

func TestEmit_FromTestdata(t *testing.T) {
	layout, err := Scan("testdata")
	require.NoError(t, err)
	require.NotNil(t, layout)

	var buf bytes.Buffer
	require.NoError(t, Emit(layout, &buf))

	out := buf.String()
	t.Logf("emitted:\n%s", out)

	var cfg config.InfraConfigV2
	require.NoError(t, yaml.Unmarshal(buf.Bytes(), &cfg), "emit must produce parseable v2 yaml")

	assert.Equal(t, "easyinfra/v2", cfg.APIVersion)
	assert.Equal(t, "k3s", cfg.Cluster.Type)
	assert.Equal(t, "testdata", cfg.Cluster.Name)
	assert.Equal(t, "testdata", cfg.Cluster.KubeContext)

	assert.Equal(t, []string{"values-shared.yaml"}, cfg.Defaults.ValueFiles)

	for _, a := range cfg.Apps {
		assert.NotEqual(t, "postgres", a.Name, "library chart must not appear as an app")
	}

	names := make([]string, 0, len(cfg.Apps))
	for _, a := range cfg.Apps {
		names = append(names, a.Name)
	}
	for i := 1; i < len(names); i++ {
		assert.LessOrEqual(t, names[i-1], names[i], "apps must be sorted alphabetically: %v", names)
	}

	assert.Contains(t, names, "caddy")
	assert.Contains(t, names, "walls")
	assert.Len(t, cfg.Apps, 2)

	var walls *config.AppConfigV2
	for i := range cfg.Apps {
		if cfg.Apps[i].Name == "walls" {
			walls = &cfg.Apps[i]
		}
	}
	require.NotNil(t, walls)
	assert.Contains(t, walls.DependsOn, "postgres")
	assert.Equal(t, "apps/walls", walls.Chart)
	assert.Equal(t, "walls", walls.Namespace)
}

func TestEmit_LibraryNotEmitted(t *testing.T) {
	layout, err := Scan("testdata")
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, Emit(layout, &buf))
	assert.NotContains(t, buf.String(), "name: postgres")
}

func TestEmit_AppsSortedAlphabetically(t *testing.T) {
	layout := &Layout{
		Root: "myroot",
		Apps: []AppLayout{
			{Name: "zebra", ChartPath: "myroot/apps/zebra"},
			{Name: "alpha", ChartPath: "myroot/apps/alpha"},
			{Name: "mango", ChartPath: "myroot/apps/mango"},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, Emit(layout, &buf))

	out := buf.String()
	iAlpha := strings.Index(out, "name: alpha")
	iMango := strings.Index(out, "name: mango")
	iZebra := strings.Index(out, "name: zebra")
	require.True(t, iAlpha >= 0 && iMango >= 0 && iZebra >= 0, "all three apps must appear: %s", out)
	assert.Less(t, iAlpha, iMango)
	assert.Less(t, iMango, iZebra)

	var cfg config.InfraConfigV2
	require.NoError(t, yaml.Unmarshal(buf.Bytes(), &cfg))
	require.Len(t, cfg.Apps, 3)
	assert.Equal(t, "alpha", cfg.Apps[0].Name)
	assert.Equal(t, "mango", cfg.Apps[1].Name)
	assert.Equal(t, "zebra", cfg.Apps[2].Name)
}

func TestEmit_PostRendererHint(t *testing.T) {
	layout, err := Scan("testdata")
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, Emit(layout, &buf))

	out := buf.String()
	assert.Contains(t, out, "postRenderer:")
	assert.Contains(t, out, "binary: obscuro")
	assert.Contains(t, out, "args: [inject]")
}

func TestEmit_Deterministic(t *testing.T) {
	layout, err := Scan("testdata")
	require.NoError(t, err)

	var a, b bytes.Buffer
	require.NoError(t, Emit(layout, &a))
	require.NoError(t, Emit(layout, &b))
	assert.Equal(t, a.String(), b.String(), "emit must be deterministic")
}

func TestEmit_NilLayout(t *testing.T) {
	var buf bytes.Buffer
	err := Emit(nil, &buf)
	assert.Error(t, err)
}
