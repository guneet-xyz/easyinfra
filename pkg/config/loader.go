package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Load reads and validates an infra.yaml file.
func Load(path string) (*InfraConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg InfraConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	baseDir := filepath.Dir(path)
	if err := Validate(&cfg, baseDir); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks the config for correctness. baseDir is the directory containing infra.yaml.
func Validate(cfg *InfraConfig, baseDir string) error {
	var errs []error

	if cfg.KubeContext == "" {
		errs = append(errs, fmt.Errorf("kubeContext is required"))
	}
	if len(cfg.Apps) == 0 {
		errs = append(errs, fmt.Errorf("at least one app is required"))
	}

	seen := make(map[string]bool)
	for _, app := range cfg.Apps {
		if app.Name != "" && seen[app.Name] {
			errs = append(errs, fmt.Errorf("duplicate app name: %q", app.Name))
		}
		seen[app.Name] = true
	}

	for _, app := range cfg.Apps {
		if app.Name == "" {
			errs = append(errs, fmt.Errorf("app has empty name"))
		}
		if app.Chart == "" {
			errs = append(errs, fmt.Errorf("app %q: chart is required", app.Name))
		}
		if app.Namespace == "" {
			errs = append(errs, fmt.Errorf("app %q: namespace is required", app.Name))
		}
		if app.Order == 0 {
			errs = append(errs, fmt.Errorf("app %q: order must be non-zero", app.Name))
		}
		if app.Chart != "" {
			chartPath := filepath.Join(baseDir, app.Chart)
			if _, err := os.Stat(chartPath); err != nil {
				errs = append(errs, fmt.Errorf("app %q: chart path %q does not exist", app.Name, chartPath))
			}
		}
		for _, vf := range app.ValueFiles {
			vfPath := filepath.Join(baseDir, vf)
			if _, err := os.Stat(vfPath); err != nil {
				errs = append(errs, fmt.Errorf("app %q: valueFile %q does not exist", app.Name, vfPath))
			}
		}
	}

	for _, vf := range cfg.Defaults.ValueFiles {
		vfPath := filepath.Join(baseDir, vf)
		if _, err := os.Stat(vfPath); err != nil {
			errs = append(errs, fmt.Errorf("defaults.valueFiles: %q does not exist", vfPath))
		}
	}

	for _, app := range cfg.Apps {
		for _, dep := range app.DependsOn {
			if !seen[dep] {
				errs = append(errs, fmt.Errorf("app %q: dependsOn references unknown app %q", app.Name, dep))
			}
		}
	}

	hasPVCs := false
	for _, app := range cfg.Apps {
		if len(app.PVCs) > 0 {
			hasPVCs = true
			break
		}
	}
	if hasPVCs && cfg.Backup.RemoteHost == "" {
		errs = append(errs, fmt.Errorf("backup.remoteHost is required when apps have pvcs"))
	}

	if cycleErr := detectCycles(cfg.Apps); cycleErr != nil {
		errs = append(errs, cycleErr)
	}

	return errors.Join(errs...)
}

// detectCycles uses DFS to find cycles in the dependsOn graph.
func detectCycles(apps []AppConfig) error {
	adj := make(map[string][]string)
	for _, app := range apps {
		adj[app.Name] = app.DependsOn
	}

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make(map[string]int)
	var path []string

	var dfs func(name string) error
	dfs = func(name string) error {
		state[name] = visiting
		path = append(path, name)
		for _, dep := range adj[name] {
			switch state[dep] {
			case visiting:
				cycleStart := -1
				for i, n := range path {
					if n == dep {
						cycleStart = i
						break
					}
				}
				cycle := append(path[cycleStart:], dep)
				return fmt.Errorf("cycle detected in dependsOn: %s", joinPath(cycle))
			case unvisited:
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		path = path[:len(path)-1]
		state[name] = visited
		return nil
	}

	for _, app := range apps {
		if state[app.Name] == unvisited {
			if err := dfs(app.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func joinPath(path []string) string {
	result := ""
	for i, p := range path {
		if i > 0 {
			result += " -> "
		}
		result += p
	}
	return result
}

// MergedPostRenderer returns the effective post-renderer for an app.
// App-level overrides defaults.
func MergedPostRenderer(app *AppConfig, cfg *InfraConfig) *PostRenderer {
	if app.PostRenderer != nil {
		return app.PostRenderer
	}
	return cfg.Defaults.PostRenderer
}

// MergedValueFiles returns the effective value files for an app.
// Defaults come first, then app-specific files. Deduped preserving order.
func MergedValueFiles(app *AppConfig, cfg *InfraConfig) []string {
	seen := make(map[string]bool)
	var result []string
	for _, vf := range cfg.Defaults.ValueFiles {
		if !seen[vf] {
			seen[vf] = true
			result = append(result, vf)
		}
	}
	for _, vf := range app.ValueFiles {
		if !seen[vf] {
			seen[vf] = true
			result = append(result, vf)
		}
	}
	return result
}

// SortedByOrder returns apps sorted by Order ascending (stable sort).
func SortedByOrder(cfg *InfraConfig) []AppConfig {
	sorted := make([]AppConfig, len(cfg.Apps))
	copy(sorted, cfg.Apps)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})
	return sorted
}
