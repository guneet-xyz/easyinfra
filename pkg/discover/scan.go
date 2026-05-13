// Package discover walks a k3s app directory layout and produces a Layout
// describing apps, library charts, value files, local file:// dependencies,
// shared values, and any post-renderer hint.
package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Layout describes the discovered structure of an infra root directory.
type Layout struct {
	Root             string
	Apps             []AppLayout
	SharedValues     []string
	PostRendererHint *PostRendererHint
}

// AppLayout describes a single chart directory found under <root>/apps/.
type AppLayout struct {
	Name       string
	ChartPath  string
	IsLibrary  bool
	ValueFiles []string
	LocalDeps  []string
}

// PostRendererHint is the optional .easyinfra-hints.yaml post-renderer config.
type PostRendererHint struct {
	Binary string
	Args   []string
}

// chartYaml is the minimal subset of Chart.yaml we parse.
type chartYaml struct {
	Name         string          `yaml:"name"`
	Type         string          `yaml:"type"`
	Dependencies []chartYamlDep  `yaml:"dependencies"`
}

type chartYamlDep struct {
	Name       string `yaml:"name"`
	Repository string `yaml:"repository"`
	Version    string `yaml:"version"`
}

type hintsFile struct {
	PostRenderer *struct {
		Binary string   `yaml:"binary"`
		Args   []string `yaml:"args"`
	} `yaml:"postRenderer"`
}

// Scan walks <root>/apps/*/Chart.yaml and returns the resulting Layout.
func Scan(root string) (*Layout, error) {
	if root == "" {
		return nil, fmt.Errorf("discover: empty root")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("discover: stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("discover: root %q is not a directory", root)
	}

	layout := &Layout{Root: root}

	appsDir := filepath.Join(root, "apps")
	if entries, err := os.ReadDir(appsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			chartDir := filepath.Join(appsDir, e.Name())
			al, ok, err := scanChartDir(chartDir)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			layout.Apps = append(layout.Apps, al)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("discover: read apps dir: %w", err)
	}

	sort.Slice(layout.Apps, func(i, j int) bool {
		return layout.Apps[i].Name < layout.Apps[j].Name
	})

	// Shared values: <root>/values-shared.yaml or <root>/values-*.shared.yaml
	rootEntries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("discover: read root: %w", err)
	}
	for _, e := range rootEntries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "values-shared.yaml" ||
			(strings.HasPrefix(name, "values-") && strings.HasSuffix(name, ".shared.yaml")) {
			layout.SharedValues = append(layout.SharedValues, filepath.Join(root, name))
		}
	}
	sort.Strings(layout.SharedValues)

	// Post-renderer hint
	hintsPath := filepath.Join(root, ".easyinfra-hints.yaml")
	if data, err := os.ReadFile(hintsPath); err == nil {
		var h hintsFile
		if err := yaml.Unmarshal(data, &h); err != nil {
			return nil, fmt.Errorf("discover: parse hints: %w", err)
		}
		if h.PostRenderer != nil && h.PostRenderer.Binary != "" {
			layout.PostRendererHint = &PostRendererHint{
				Binary: h.PostRenderer.Binary,
				Args:   h.PostRenderer.Args,
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("discover: read hints: %w", err)
	}

	return layout, nil
}

// scanChartDir parses Chart.yaml in chartDir and collects values + local deps.
// Returns (layout, ok, err); ok=false when the dir has no Chart.yaml.
func scanChartDir(chartDir string) (AppLayout, bool, error) {
	chartPath := filepath.Join(chartDir, "Chart.yaml")
	data, err := os.ReadFile(chartPath)
	if err != nil {
		if os.IsNotExist(err) {
			return AppLayout{}, false, nil
		}
		return AppLayout{}, false, fmt.Errorf("discover: read %s: %w", chartPath, err)
	}
	var cy chartYaml
	if err := yaml.Unmarshal(data, &cy); err != nil {
		return AppLayout{}, false, fmt.Errorf("discover: parse %s: %w", chartPath, err)
	}
	name := cy.Name
	if name == "" {
		name = filepath.Base(chartDir)
	}

	al := AppLayout{
		Name:      name,
		ChartPath: chartDir,
		IsLibrary: strings.EqualFold(cy.Type, "library"),
	}

	// Value files: values.yaml + values-*.yaml in chartDir.
	entries, err := os.ReadDir(chartDir)
	if err != nil {
		return AppLayout{}, false, fmt.Errorf("discover: read %s: %w", chartDir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if n == "values.yaml" || (strings.HasPrefix(n, "values-") && strings.HasSuffix(n, ".yaml")) {
			al.ValueFiles = append(al.ValueFiles, filepath.Join(chartDir, n))
		}
	}
	sort.Strings(al.ValueFiles)

	// Local file:// deps.
	for _, dep := range cy.Dependencies {
		repo := strings.TrimSpace(dep.Repository)
		if !strings.HasPrefix(repo, "file://") {
			continue
		}
		al.LocalDeps = append(al.LocalDeps, strings.TrimPrefix(repo, "file://"))
	}

	return al, true, nil
}
