// Package deps provides helm chart dependency management with file:// awareness.
package deps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"gopkg.in/yaml.v3"
)

// Issue describes a dependency problem discovered by Check.
type Issue struct {
	App     string
	Kind    string // "missing-lock" | "lock-stale" | "file-dep-missing"
	Message string
}

type dependency struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	Repository string `yaml:"repository"`
}

type chartYaml struct {
	Name         string       `yaml:"name"`
	Dependencies []dependency `yaml:"dependencies"`
}

type chartLock struct {
	Dependencies []dependency `yaml:"dependencies"`
}

// Update shells out to `helm dependency update <chartDir>`.
func Update(ctx context.Context, runner exec.Runner, chartDir string) error {
	_, stderr, err := runner.Run(ctx, "helm", "dependency", "update", chartDir)
	if err != nil {
		return fmt.Errorf("helm dependency update %s: %w\n%s", chartDir, err, stderr)
	}
	return nil
}

// Check inspects Chart.yaml/Chart.lock and file:// dependencies for the given chart directory.
func Check(_ context.Context, chartDir string) ([]Issue, error) {
	chartYamlPath := filepath.Join(chartDir, "Chart.yaml")
	data, err := os.ReadFile(chartYamlPath)
	if err != nil {
		return nil, fmt.Errorf("read Chart.yaml: %w", err)
	}
	var chart chartYaml
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return nil, fmt.Errorf("parse Chart.yaml: %w", err)
	}

	app := chart.Name
	if app == "" {
		app = filepath.Base(chartDir)
	}

	var issues []Issue

	if len(chart.Dependencies) > 0 {
		lockPath := filepath.Join(chartDir, "Chart.lock")
		lockData, lockErr := os.ReadFile(lockPath)
		if os.IsNotExist(lockErr) {
			issues = append(issues, Issue{
				App:     app,
				Kind:    "missing-lock",
				Message: "Chart.lock not found",
			})
		} else if lockErr != nil {
			return nil, fmt.Errorf("read Chart.lock: %w", lockErr)
		} else {
			var lock chartLock
			if err := yaml.Unmarshal(lockData, &lock); err != nil {
				return nil, fmt.Errorf("parse Chart.lock: %w", err)
			}
			if !sameDepNames(chart.Dependencies, lock.Dependencies) {
				issues = append(issues, Issue{
					App:     app,
					Kind:    "lock-stale",
					Message: "Chart.lock dependencies do not match Chart.yaml",
				})
			}
		}
	}

	for _, dep := range chart.Dependencies {
		if !strings.HasPrefix(dep.Repository, "file://") {
			continue
		}
		rel := strings.TrimPrefix(dep.Repository, "file://")
		resolved := rel
		if !filepath.IsAbs(rel) {
			resolved = filepath.Join(chartDir, rel)
		}
		if _, statErr := os.Stat(resolved); os.IsNotExist(statErr) {
			issues = append(issues, Issue{
				App:     app,
				Kind:    "file-dep-missing",
				Message: fmt.Sprintf("local dep not found: %s", resolved),
			})
		} else if statErr != nil {
			return nil, fmt.Errorf("stat %s: %w", resolved, statErr)
		}
	}

	return issues, nil
}

func sameDepNames(a, b []dependency) bool {
	if len(a) != len(b) {
		return false
	}
	an := make([]string, len(a))
	bn := make([]string, len(b))
	for i, d := range a {
		an[i] = d.Name
	}
	for i, d := range b {
		bn[i] = d.Name
	}
	sort.Strings(an)
	sort.Strings(bn)
	for i := range an {
		if an[i] != bn[i] {
			return false
		}
	}
	return true
}
