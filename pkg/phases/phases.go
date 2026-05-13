// Package phases resolves apps into ordered install phases.
//
// A phase is a group of apps that should be installed together; phases are
// installed sequentially, while apps within a phase are topologically sorted
// by their DependsOn edges.
//
// Apps not assigned to any phase fall into a synthetic terminal phase named
// "default".
package phases

import (
	"fmt"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/topo"
)

// DefaultPhaseName is the name of the synthetic terminal phase that holds
// apps not explicitly assigned to any phase in cfg.Phases.
const DefaultPhaseName = "default"

// Resolve returns the apps grouped into phases, in install order.
//
// The returned slice contains one entry per phase (in the order phases appear
// in cfg.Phases, with the synthetic "default" phase appended last if it is
// non-empty). Each phase's apps are topologically sorted using their
// DependsOn edges.
//
// Validation:
//   - Every app referenced in cfg.Phases[].apps must exist in cfg.Apps.
//   - No app may appear in more than one phase.
func Resolve(cfg *config.InfraConfigV2) ([][]config.AppConfigV2, error) {
	if cfg == nil {
		return nil, fmt.Errorf("phases: nil config")
	}

	// Index apps by name for O(1) lookup.
	appByName := make(map[string]config.AppConfigV2, len(cfg.Apps))
	for _, app := range cfg.Apps {
		appByName[app.Name] = app
	}

	// Track which phase each app has been assigned to (for duplicate detection).
	assigned := make(map[string]string, len(cfg.Apps))

	result := make([][]config.AppConfigV2, 0, len(cfg.Phases)+1)

	for _, phase := range cfg.Phases {
		phaseApps := make([]config.AppConfigV2, 0, len(phase.Apps))
		for _, name := range phase.Apps {
			app, ok := appByName[name]
			if !ok {
				return nil, fmt.Errorf("phases: phase %q references unknown app %q", phase.Name, name)
			}
			if prev, dup := assigned[name]; dup {
				return nil, fmt.Errorf("phases: app %q appears in multiple phases (%q and %q)", name, prev, phase.Name)
			}
			assigned[name] = phase.Name
			phaseApps = append(phaseApps, app)
		}

		sorted, err := sortPhase(phaseApps)
		if err != nil {
			return nil, fmt.Errorf("phases: phase %q: %w", phase.Name, err)
		}
		result = append(result, sorted)
	}

	// Collect unassigned apps into the synthetic "default" phase, preserving
	// the order they appear in cfg.Apps.
	var defaults []config.AppConfigV2
	for _, app := range cfg.Apps {
		if _, ok := assigned[app.Name]; ok {
			continue
		}
		defaults = append(defaults, app)
	}

	if len(defaults) > 0 {
		sorted, err := sortPhase(defaults)
		if err != nil {
			return nil, fmt.Errorf("phases: phase %q: %w", DefaultPhaseName, err)
		}
		result = append(result, sorted)
	}

	return result, nil
}

// sortPhase topologically sorts apps within a phase using DependsOn edges.
// Edges to apps outside the phase are ignored by topo.Sort (unknown nodes).
func sortPhase(apps []config.AppConfigV2) ([]config.AppConfigV2, error) {
	if len(apps) == 0 {
		return []config.AppConfigV2{}, nil
	}

	names := make([]string, 0, len(apps))
	edges := make(map[string][]string, len(apps))
	byName := make(map[string]config.AppConfigV2, len(apps))
	for _, app := range apps {
		names = append(names, app.Name)
		edges[app.Name] = app.DependsOn
		byName[app.Name] = app
	}

	ordered, cycErr := topo.Sort(names, edges)
	if cycErr != nil {
		return nil, cycErr
	}

	out := make([]config.AppConfigV2, 0, len(ordered))
	for _, n := range ordered {
		out = append(out, byName[n])
	}
	return out, nil
}
