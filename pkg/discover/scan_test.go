package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan(t *testing.T) {
	layout, err := Scan("testdata")
	require.NoError(t, err)
	require.NotNil(t, layout)

	byName := map[string]AppLayout{}
	for _, a := range layout.Apps {
		byName[a.Name] = a
	}

	require.Len(t, layout.Apps, 3, "expected caddy, postgres, walls")

	caddy, ok := byName["caddy"]
	require.True(t, ok)
	assert.False(t, caddy.IsLibrary)
	assert.ElementsMatch(t,
		[]string{
			filepath.Join("testdata/apps/caddy", "values.yaml"),
			filepath.Join("testdata/apps/caddy", "values-prod.yaml"),
		},
		caddy.ValueFiles,
	)

	pg, ok := byName["postgres"]
	require.True(t, ok)
	assert.True(t, pg.IsLibrary, "postgres should be marked as library")

	walls, ok := byName["walls"]
	require.True(t, ok)
	assert.False(t, walls.IsLibrary)
	assert.Equal(t, []string{"../../shared/postgres"}, walls.LocalDeps)

	libCount := 0
	for _, a := range layout.Apps {
		if a.IsLibrary {
			libCount++
		}
	}
	assert.Equal(t, 1, libCount, "expected exactly 1 library chart")
	assert.Equal(t, 2, len(layout.Apps)-libCount, "expected 2 non-library apps")

	require.Len(t, layout.SharedValues, 1)
	assert.Equal(t, filepath.Join("testdata", "values-shared.yaml"), layout.SharedValues[0])

	require.NotNil(t, layout.PostRendererHint)
	assert.Equal(t, "obscuro", layout.PostRendererHint.Binary)
	assert.Equal(t, []string{"inject"}, layout.PostRendererHint.Args)
}

func TestScan_MissingRoot(t *testing.T) {
	_, err := Scan("testdata/does-not-exist")
	assert.Error(t, err)
}

func TestScan_NoHints(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "apps"), 0o755))
	layout, err := Scan(dir)
	require.NoError(t, err)
	assert.Nil(t, layout.PostRendererHint)
	assert.Empty(t, layout.Apps)
	assert.Empty(t, layout.SharedValues)
}
