package config

import (
	"fmt"

	"github.com/guneet-xyz/easyinfra/pkg/topo"
)

// MigrateV1ToV2ForOrdering exposes the in-memory v1→v2 migration so that
// callers needing only ordering (install/upgrade/uninstall) can sort v1
// configs through SortedByDependency without re-implementing the mapping.
func MigrateV1ToV2ForOrdering(v1 *InfraConfig) *InfraConfigV2 {
	return migrateV1ToV2(v1)
}

// SortedByDependency returns apps in dependency order (prerequisites first).
//
// Edges are derived from each app's DependsOn field. Ties between independent
// apps are broken by (Phase index ascending, Order ascending, Name ascending).
// If the dependency graph contains a cycle, a *topo.CycleError is returned.
func SortedByDependency(cfg *InfraConfigV2) ([]AppConfigV2, error) {
	if cfg == nil || len(cfg.Apps) == 0 {
		return []AppConfigV2{}, nil
	}

	appByName := make(map[string]AppConfigV2, len(cfg.Apps))
	for _, a := range cfg.Apps {
		appByName[a.Name] = a
	}

	const noPhase = 1 << 30
	phaseIdx := make(map[string]int, len(cfg.Apps))
	for _, a := range cfg.Apps {
		phaseIdx[a.Name] = noPhase
	}
	for i, ph := range cfg.Phases {
		for _, name := range ph.Apps {
			if _, ok := phaseIdx[name]; ok {
				phaseIdx[name] = i
			}
		}
	}

	// Encode tie-breakers (phase index, order, name) into the node id so that
	// topo.Sort's alphabetical tie-breaking yields the desired ordering.
	keyOf := func(a AppConfigV2) string {
		return fmt.Sprintf("%010d|%010d|%s", phaseIdx[a.Name], a.Order, a.Name)
	}

	keyToName := make(map[string]string, len(cfg.Apps))
	nodes := make([]string, 0, len(cfg.Apps))
	for _, a := range cfg.Apps {
		k := keyOf(a)
		nodes = append(nodes, k)
		keyToName[k] = a.Name
	}

	edges := make(map[string][]string, len(cfg.Apps))
	for _, a := range cfg.Apps {
		k := keyOf(a)
		for _, dep := range a.DependsOn {
			depApp, ok := appByName[dep]
			if !ok {
				continue
			}
			edges[k] = append(edges[k], keyOf(depApp))
		}
	}

	ordered, cycErr := topo.Sort(nodes, edges)
	if cycErr != nil {
		path := make([]string, len(cycErr.Path))
		for i, k := range cycErr.Path {
			if name, ok := keyToName[k]; ok {
				path[i] = name
			} else {
				path[i] = k
			}
		}
		return nil, &topo.CycleError{Path: path}
	}

	out := make([]AppConfigV2, 0, len(ordered))
	for _, k := range ordered {
		out = append(out, appByName[keyToName[k]])
	}
	return out, nil
}
