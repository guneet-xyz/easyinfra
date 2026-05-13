// Package topo provides deterministic topological sorting with cycle detection.
package topo

import (
	"sort"
	"strings"
)

// CycleError is returned by Sort when the input graph contains a cycle.
// Path contains the nodes participating in the cycle, with the first node
// repeated at the end (e.g. ["a", "b", "c", "a"]).
type CycleError struct {
	Path []string
}

// Error implements the error interface.
func (e *CycleError) Error() string {
	return "cycle detected: " + strings.Join(e.Path, " → ")
}

// Sort returns a deterministic topological ordering of nodes.
//
// edges maps a node to the list of nodes it depends on (its prerequisites).
// For example, edges["walls"] = ["caddy"] means "caddy" must come before "walls".
//
// Ties are broken alphabetically by node name. If the graph contains a cycle,
// Sort returns a non-nil *CycleError describing one cycle.
func Sort(nodes []string, edges map[string][]string) ([]string, *CycleError) {
	if len(nodes) == 0 {
		return []string{}, nil
	}

	// Build node set for membership checks and ignore edges to/from unknown nodes.
	nodeSet := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		nodeSet[n] = struct{}{}
	}

	// Build adjacency list: prereq -> dependents (edge prereq → dependent).
	// Also compute in-degree for each node = number of prerequisites it has.
	adj := make(map[string][]string, len(nodes))
	indeg := make(map[string]int, len(nodes))
	for _, n := range nodes {
		indeg[n] = 0
	}
	for _, n := range nodes {
		// Deduplicate prerequisites for this node to avoid double-counting in-degree.
		seen := make(map[string]struct{})
		for _, dep := range edges[n] {
			if _, ok := nodeSet[dep]; !ok {
				continue
			}
			if _, dup := seen[dep]; dup {
				continue
			}
			seen[dep] = struct{}{}
			adj[dep] = append(adj[dep], n)
			indeg[n]++
		}
	}

	// Initialize ready set with all in-degree-0 nodes.
	var ready []string
	for _, n := range nodes {
		if indeg[n] == 0 {
			ready = append(ready, n)
		}
	}
	sort.Strings(ready)

	result := make([]string, 0, len(nodes))
	for len(ready) > 0 {
		// Pop alphabetically smallest.
		n := ready[0]
		ready = ready[1:]
		result = append(result, n)

		// Decrement in-degree of dependents; collect newly-ready ones.
		dependents := adj[n]
		sort.Strings(dependents)
		var newlyReady []string
		for _, d := range dependents {
			indeg[d]--
			if indeg[d] == 0 {
				newlyReady = append(newlyReady, d)
			}
		}
		if len(newlyReady) > 0 {
			ready = append(ready, newlyReady...)
			sort.Strings(ready)
		}
	}

	if len(result) == len(nodes) {
		return result, nil
	}

	// Cycle exists among the nodes still having in-degree > 0.
	// Reconstruct a cycle path via DFS over the residual graph.
	residual := make(map[string][]string)
	residualNodes := make([]string, 0)
	for _, n := range nodes {
		if indeg[n] > 0 {
			residualNodes = append(residualNodes, n)
		}
	}
	sort.Strings(residualNodes)
	residualSet := make(map[string]struct{}, len(residualNodes))
	for _, n := range residualNodes {
		residualSet[n] = struct{}{}
	}
	for _, n := range residualNodes {
		// Walk this node's prerequisites that are also in the residual.
		prereqs := append([]string(nil), edges[n]...)
		sort.Strings(prereqs)
		for _, dep := range prereqs {
			if _, ok := residualSet[dep]; ok {
				residual[n] = append(residual[n], dep)
			}
		}
	}

	cyclePath := findCycle(residualNodes, residual)
	return nil, &CycleError{Path: cyclePath}
}

// findCycle returns a cycle path in the given graph (which is guaranteed to
// contain at least one cycle). The returned slice has the start node repeated
// at the end, e.g. ["a", "b", "c", "a"].
func findCycle(nodes []string, adj map[string][]string) []string {
	const (
		unvisited = 0
		onStack   = 1
		done      = 2
	)
	state := make(map[string]int, len(nodes))
	var stack []string
	var cycle []string

	var dfs func(n string) bool
	dfs = func(n string) bool {
		state[n] = onStack
		stack = append(stack, n)
		neighbors := append([]string(nil), adj[n]...)
		sort.Strings(neighbors)
		for _, m := range neighbors {
			switch state[m] {
			case onStack:
				start := -1
				for i, s := range stack {
					if s == m {
						start = i
						break
					}
				}
				cycle = append([]string{}, stack[start:]...)
				cycle = append(cycle, m)
				return true
			case unvisited:
				if dfs(m) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = done
		return false
	}

	for _, n := range nodes {
		if state[n] == unvisited {
			if dfs(n) {
				return cycle
			}
		}
	}
	// Should not happen if caller verified a cycle exists; return nodes as fallback.
	return nodes
}
