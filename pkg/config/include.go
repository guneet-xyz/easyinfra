package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type includeFile struct {
	Defaults *Defaults     `yaml:"defaults,omitempty"`
	Apps     []AppConfigV2 `yaml:"apps,omitempty"`
	Includes []string      `yaml:"includes,omitempty"`
}

// IncludesResolve expands the given glob patterns relative to baseDir, parses
// each matched YAML file into a PartialConfig, and recursively resolves any
// nested `includes:` declarations.
//
// Patterns are expanded with filepath.Glob and matches are processed in
// lexical (sorted) order for deterministic output. The function errors on:
//   - a pattern that fails to expand
//   - a glob that matches zero files (treated as a missing-file error)
//   - a referenced file that cannot be read or parsed
//   - an include cycle (the same absolute path visited twice)
//   - a duplicate app name appearing in two different partials
func IncludesResolve(baseDir string, patterns []string) ([]PartialConfig, error) {
	visited := make(map[string]bool)
	seenApp := make(map[string]string) // app name -> source path
	var out []PartialConfig
	if err := resolveIncludes(baseDir, patterns, visited, seenApp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func resolveIncludes(
	baseDir string,
	patterns []string,
	visited map[string]bool,
	seenApp map[string]string,
	out *[]PartialConfig,
) error {
	for _, pattern := range patterns {
		full := pattern
		if !filepath.IsAbs(full) {
			full = filepath.Join(baseDir, pattern)
		}

		matches, err := filepath.Glob(full)
		if err != nil {
			return fmt.Errorf("includes: invalid glob %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return fmt.Errorf("includes: no files match %q (resolved to %q)", pattern, full)
		}
		sort.Strings(matches)

		for _, match := range matches {
			abs, err := filepath.Abs(match)
			if err != nil {
				return fmt.Errorf("includes: resolving %q: %w", match, err)
			}
			if visited[abs] {
				return fmt.Errorf("include cycle detected: %s", abs)
			}
			visited[abs] = true

			data, err := os.ReadFile(abs)
			if err != nil {
				return fmt.Errorf("includes: reading %s: %w", abs, err)
			}
			var f includeFile
			if err := yaml.Unmarshal(data, &f); err != nil {
				return fmt.Errorf("includes: parsing %s: %w", abs, err)
			}

			for _, app := range f.Apps {
				if prev, ok := seenApp[app.Name]; ok {
					return fmt.Errorf("duplicate app %q: defined in %q and %q", app.Name, prev, abs)
				}
				seenApp[app.Name] = abs
			}

			*out = append(*out, PartialConfig{
				Source:   abs,
				Defaults: f.Defaults,
				Apps:     f.Apps,
			})

			// Recurse into nested includes; nested patterns are relative to
			// the directory of the current included file.
			if len(f.Includes) > 0 {
				nestedBase := filepath.Dir(abs)
				if err := resolveIncludes(nestedBase, f.Includes, visited, seenApp, out); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
