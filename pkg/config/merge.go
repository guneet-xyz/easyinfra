// Package config provides configuration loading and merging for easyinfra.
package config

import "fmt"

// MergeAppDefaultsV2 returns the effective configuration for an app by merging
// global defaults with app-specific overrides.
//
// Rules (from plan §"Composition merge semantics"):
//  1. Value-file lists are APPENDED (defaults first, then app-specific). Deduped preserving order.
//  2. PostRenderer: app-level overrides defaults entirely (not merged field-by-field).
//  3. App-local files cannot remove a key from defaults; they can only override scalar values or extend lists.
func MergeAppDefaultsV2(app *AppConfigV2, defaults *Defaults) AppConfigV2 {
	result := *app // copy

	// Rule 1: value files — defaults first, then app-specific, deduped
	result.ValueFiles = mergeValueFiles(defaults.ValueFiles, app.ValueFiles)

	// Rule 2: post-renderer — app overrides defaults entirely
	if result.PostRenderer == nil && defaults.PostRenderer != nil {
		pr := *defaults.PostRenderer // copy to avoid mutation
		result.PostRenderer = &pr
	}

	// Rule 3: namespace defaults to name if empty
	if result.Namespace == "" {
		result.Namespace = app.Name
	}

	return result
}

// mergeValueFiles merges two value file lists: base first, then extra.
// Deduplication preserves order (first occurrence wins).
func mergeValueFiles(base, extra []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, vf := range base {
		if !seen[vf] {
			seen[vf] = true
			result = append(result, vf)
		}
	}
	for _, vf := range extra {
		if !seen[vf] {
			seen[vf] = true
			result = append(result, vf)
		}
	}
	return result
}

// MergeIncludesV2 merges a base InfraConfigV2 with a slice of partial configs
// loaded from includes. Returns an error if any app name appears in more than one source.
//
// Rules:
//   - Conflict on duplicate app name across includes is an error (no last-writer-wins).
//   - Defaults from includes are merged into base defaults (value files appended, postrenderer: base wins).
//   - Apps from includes are appended to base apps (after dedup check).
func MergeIncludesV2(base *InfraConfigV2, partials []PartialConfig) (*InfraConfigV2, error) {
	result := *base // shallow copy

	// Track app names for conflict detection
	seen := make(map[string]string) // name → source
	for _, app := range base.Apps {
		seen[app.Name] = "base"
	}

	for _, partial := range partials {
		// Merge defaults: append value files, base postrenderer wins
		if partial.Defaults != nil {
			result.Defaults.ValueFiles = mergeValueFiles(result.Defaults.ValueFiles, partial.Defaults.ValueFiles)
			// PostRenderer: base wins (do not override)
		}

		// Merge apps: conflict check
		for _, app := range partial.Apps {
			if existing, ok := seen[app.Name]; ok {
				return nil, fmt.Errorf("duplicate app %q: defined in %q and %q", app.Name, existing, partial.Source)
			}
			seen[app.Name] = partial.Source
			result.Apps = append(result.Apps, app)
		}
	}

	return &result, nil
}

// PartialConfig is a fragment loaded from an includes file.
type PartialConfig struct {
	Source   string        // file path for error messages
	Defaults *Defaults     // optional defaults override
	Apps     []AppConfigV2 // apps defined in this fragment
}
