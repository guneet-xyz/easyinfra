package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompose exercises the configuration composition pipeline:
// IncludesResolve → MergeIncludesV2 → MergeAppDefaultsV2, plus
// LoadV2 convention-based discovery.
//
// Each sub-test focuses on one composition rule from the plan
// ("Composition merge semantics") and uses dedicated YAML fixtures
// under testdata/composition/.
func TestCompose(t *testing.T) {
	t.Run("defaults-only", func(t *testing.T) {
		// No apps, only defaults: merging with no partials must preserve
		// defaults verbatim and yield an empty apps list.
		base := &InfraConfigV2{
			APIVersion: APIVersionV2,
			Defaults: Defaults{
				ValueFiles:   []string{"shared.yaml"},
				PostRenderer: &PostRenderer{Command: "obscuro", Args: []string{"inject"}},
			},
		}
		merged, err := MergeIncludesV2(base, nil)
		require.NoError(t, err)
		assert.Empty(t, merged.Apps, "no partials → no apps")
		assert.Equal(t, []string{"shared.yaml"}, merged.Defaults.ValueFiles)
		require.NotNil(t, merged.Defaults.PostRenderer)
		assert.Equal(t, "obscuro", merged.Defaults.PostRenderer.Command)
	})

	t.Run("includes-only", func(t *testing.T) {
		// Two include files each contributing one app → 2 apps loaded.
		parts, err := IncludesResolve("testdata/composition/includes-only", []string{"*.yaml"})
		require.NoError(t, err)
		require.Len(t, parts, 2)

		base := &InfraConfigV2{APIVersion: APIVersionV2}
		merged, err := MergeIncludesV2(base, parts)
		require.NoError(t, err)
		assert.Len(t, merged.Apps, 2)

		names := []string{merged.Apps[0].Name, merged.Apps[1].Name}
		assert.ElementsMatch(t, []string{"alpha", "beta"}, names)
	})

	t.Run("defaults-and-includes", func(t *testing.T) {
		// Base defaults must propagate into apps loaded from includes.
		base := &InfraConfigV2{
			APIVersion: APIVersionV2,
			Defaults: Defaults{
				ValueFiles: []string{"shared.yaml"},
			},
		}
		parts, err := IncludesResolve("testdata/composition/includes-only", []string{"one.yaml"})
		require.NoError(t, err)

		merged, err := MergeIncludesV2(base, parts)
		require.NoError(t, err)
		require.Len(t, merged.Apps, 1)

		eff := MergeAppDefaultsV2(&merged.Apps[0], &merged.Defaults)
		assert.Equal(t, []string{"shared.yaml"}, eff.ValueFiles,
			"defaults.valueFiles must propagate to merged app")
	})

	t.Run("conflicting-includes", func(t *testing.T) {
		// Two includes that both define the same app name → error.
		_, err := IncludesResolve("testdata/composition/conflicting", []string{"one.yaml", "two.yaml"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate app")
		assert.Contains(t, err.Error(), "shared")
	})

	t.Run("cycle-detection", func(t *testing.T) {
		// a.yaml includes b.yaml which includes a.yaml → cycle error.
		_, err := IncludesResolve("testdata/composition/cycle", []string{"a.yaml"})
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "cycle")
	})

	t.Run("postrenderer-override", func(t *testing.T) {
		// App-level postRenderer must replace defaults entirely.
		defaults := &Defaults{
			PostRenderer: &PostRenderer{Command: "obscuro", Args: []string{"inject"}},
		}
		app := &AppConfigV2{
			Name:         "walls",
			Chart:        "apps/walls",
			PostRenderer: &PostRenderer{Command: "custom", Args: []string{"run"}},
		}
		eff := MergeAppDefaultsV2(app, defaults)
		require.NotNil(t, eff.PostRenderer)
		assert.Equal(t, "custom", eff.PostRenderer.Command,
			"app-level postRenderer must override defaults")
		assert.Equal(t, []string{"run"}, eff.PostRenderer.Args)
	})

	t.Run("value-files-extend", func(t *testing.T) {
		// Defaults valueFiles are prepended; app valueFiles appended.
		defaults := &Defaults{ValueFiles: []string{"shared.yaml", "common.yaml"}}
		app := &AppConfigV2{
			Name:       "walls",
			Chart:      "apps/walls",
			ValueFiles: []string{"walls.yaml"},
		}
		eff := MergeAppDefaultsV2(app, defaults)
		assert.Equal(t,
			[]string{"shared.yaml", "common.yaml", "walls.yaml"},
			eff.ValueFiles,
			"defaults first, then app-specific (deduped, order preserved)")
	})

	t.Run("missing-include-file", func(t *testing.T) {
		// Reference to nonexistent file → error.
		_, err := IncludesResolve(
			"testdata/composition/includes-only",
			[]string{"does-not-exist.yaml"},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no files match")
	})

	t.Run("glob-expansion", func(t *testing.T) {
		// Glob must expand in lexical (sorted) order.
		parts, err := IncludesResolve("testdata/composition/glob", []string{"*.yaml"})
		require.NoError(t, err)
		require.Len(t, parts, 3)

		wantOrder := []string{"a.yaml", "b.yaml", "c.yaml"}
		for i, p := range parts {
			assert.Truef(t, strings.HasSuffix(p.Source, wantOrder[i]),
				"partial %d: source %q does not end with %q", i, p.Source, wantOrder[i])
		}
		// Apps follow the same lexical ordering.
		wantApps := []string{"alpha", "beta", "gamma"}
		for i, p := range parts {
			require.Lenf(t, p.Apps, 1, "partial %d", i)
			assert.Equal(t, wantApps[i], p.Apps[0].Name)
		}
	})

	t.Run("convention-mode", func(t *testing.T) {
		// No apps and no includes → LoadV2 must auto-discover
		// apps/*/easyinfra.yaml in lexical order.
		cfg, err := LoadV2("testdata/composition/convention/infra.yaml")
		require.NoError(t, err)
		require.Len(t, cfg.Apps, 2, "expected 2 discovered apps")

		names := []string{cfg.Apps[0].Name, cfg.Apps[1].Name}
		assert.Equal(t, []string{"caddy", "walls"}, names,
			"discovery must yield lexical ordering")
	})
}
