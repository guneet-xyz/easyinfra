package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Quiet suppresses deprecation warnings when set to true.
var Quiet bool

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
	emitV1DeprecationWarning(&cfg)
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

// MergedPostRendererV2 returns the effective post-renderer for a v2 app.
// App-level overrides defaults.
func MergedPostRendererV2(app *AppConfigV2, cfg *InfraConfigV2) *PostRenderer {
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

// MergedValueFilesV2 returns the effective value files for a v2 app.
// Defaults come first, then app-specific files. Deduped preserving order.
func MergedValueFilesV2(app *AppConfigV2, cfg *InfraConfigV2) []string {
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
//
// Deprecated: use SortedByDependency, which respects dependsOn and uses
// Order only as a tie-breaker.
func SortedByOrder(cfg *InfraConfig) []AppConfig {
	sorted := make([]AppConfig, len(cfg.Apps))
	copy(sorted, cfg.Apps)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})
	return sorted
}

// LoadV1 reads and validates an infra.yaml file as v1 (transitional alias for Load).
func LoadV1(path string) (*InfraConfig, error) {
	return Load(path)
}

// LoadV2 reads an infra.yaml file and returns a v2 config. v1 files are
// auto-migrated in-memory and a deprecation warning is emitted to stderr.
func LoadV2(path string) (*InfraConfigV2, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	baseDir := filepath.Dir(path)

	switch DetectAPIVersion(data) {
	case APIVersionV2:
		var cfg InfraConfigV2
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}
		if len(cfg.Apps) == 0 && len(cfg.Includes) == 0 {
			discovered, err := discoverAppLocals(baseDir)
			if err != nil {
				return nil, err
			}
			cfg.Apps = append(cfg.Apps, discovered...)
		}
		applyV2Defaults(&cfg)
		if err := ValidateV2(&cfg, baseDir); err != nil {
			return nil, err
		}
		return &cfg, nil
	default:
		var v1 InfraConfig
		if err := yaml.Unmarshal(data, &v1); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}

		if err := Validate(&v1, baseDir); err != nil {
			return nil, err
		}
		emitV1DeprecationWarning(&v1)
		return MigrateV1ToV2ForOrdering(&v1), nil
	}
}

// LoadV2SkipChartPaths loads a v2 config but skips chart path validation.
// This is useful for checking kubeContext before validating file paths.
func LoadV2SkipChartPaths(path string) (*InfraConfigV2, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	switch DetectAPIVersion(data) {
	case APIVersionV2:
		var cfg InfraConfigV2
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}
		applyV2Defaults(&cfg)
		if err := ValidateV2SkipChartPaths(&cfg); err != nil {
			return nil, err
		}
		return &cfg, nil
	default:
		var v1 InfraConfig
		if err := yaml.Unmarshal(data, &v1); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}

		return MigrateV1ToV2ForOrdering(&v1), nil
	}
}

// migrateV1ToV2 converts a v1 config into an in-memory v2 config.
func migrateV1ToV2(v1 *InfraConfig) *InfraConfigV2 {
	apps := make([]AppConfigV2, len(v1.Apps))
	for i, a := range v1.Apps {
		apps[i] = AppConfigV2(a)
	}
	return &InfraConfigV2{
		APIVersion: APIVersionV2,
		Cluster: ClusterConfig{
			Name:        v1.KubeContext,
			Type:        "k3s",
			KubeContext: v1.KubeContext,
		},
		Defaults: v1.Defaults,
		Backup: BackupConfigV2{
			RemoteHost: v1.Backup.RemoteHost,
			RemoteUser: v1.Backup.RemoteUser,
			RemoteTmp:  v1.Backup.RemoteTmp,
			LocalDir:   v1.Backup.LocalDir,
		},
		Apps: apps,
	}
}

// applyV2Defaults fills in computed defaults (e.g. namespace = name).
func applyV2Defaults(cfg *InfraConfigV2) {
	for i := range cfg.Apps {
		if cfg.Apps[i].Namespace == "" {
			cfg.Apps[i].Namespace = cfg.Apps[i].Name
		}
	}
}

// emitV1DeprecationWarning writes deprecation messages to stderr.
func emitV1DeprecationWarning(v1 *InfraConfig) {
	if Quiet {
		return
	}
	fmt.Fprintln(os.Stderr, "DEPRECATED: missing apiVersion; assuming v1. Add 'apiVersion: easyinfra/v2' to suppress this warning.")
	var ordered []string
	for _, a := range v1.Apps {
		if a.Order != 0 {
			ordered = append(ordered, a.Name)
		}
	}
	if len(ordered) > 0 {
		fmt.Fprintf(os.Stderr, "DEPRECATED: 'order' field is deprecated; declare 'dependsOn' instead. Affected apps: %s\n", strings.Join(ordered, ", "))
	}
}

// ValidateV2 checks a v2 config for correctness. baseDir is the directory containing the config file.
func ValidateV2(cfg *InfraConfigV2, baseDir string) error {
	var errs []error

	if cfg.Cluster.KubeContext == "" {
		errs = append(errs, fmt.Errorf("cluster.kubeContext is required"))
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

	if cycleErr := detectCyclesV2(cfg.Apps); cycleErr != nil {
		errs = append(errs, cycleErr)
	}

	return errors.Join(errs...)
}

// ValidateV2SkipChartPaths validates a v2 config but skips chart path existence checks.
// This is useful for checking kubeContext before validating file paths.
func ValidateV2SkipChartPaths(cfg *InfraConfigV2) error {
	var errs []error

	if cfg.Cluster.KubeContext == "" {
		errs = append(errs, fmt.Errorf("cluster.kubeContext is required"))
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

	if cycleErr := detectCyclesV2(cfg.Apps); cycleErr != nil {
		errs = append(errs, cycleErr)
	}

	return errors.Join(errs...)
}

// detectCyclesV2 mirrors detectCycles for v2 apps.
func detectCyclesV2(apps []AppConfigV2) error {
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

func discoverAppLocals(baseDir string) ([]AppConfigV2, error) {
	pattern := filepath.Join(baseDir, "apps", "*", "easyinfra.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("convention discovery: invalid glob %q: %w", pattern, err)
	}
	sort.Strings(matches)

	out := make([]AppConfigV2, 0, len(matches))
	for _, match := range matches {
		data, err := os.ReadFile(match)
		if err != nil {
			return nil, fmt.Errorf("convention discovery: reading %s: %w", match, err)
		}
		var local AppLocal
		if err := yaml.Unmarshal(data, &local); err != nil {
			return nil, fmt.Errorf("convention discovery: parsing %s: %w", match, err)
		}

		appDir := filepath.Dir(match)
		relAppDir, err := filepath.Rel(baseDir, appDir)
		if err != nil {
			return nil, fmt.Errorf("convention discovery: relative path for %s: %w", appDir, err)
		}

		valueFiles := make([]string, len(local.ValueFiles))
		for i, vf := range local.ValueFiles {
			if filepath.IsAbs(vf) {
				valueFiles[i] = vf
			} else {
				valueFiles[i] = filepath.Join(relAppDir, vf)
			}
		}

		out = append(out, AppConfigV2{
			Name:         local.Name,
			Chart:        local.Chart,
			ValueFiles:   valueFiles,
			PostRenderer: local.PostRenderer,
			DependsOn:    local.DependsOn,
			PVCs:         local.PVCs,
		})
	}
	return out, nil
}
