package topo

import (
	"reflect"
	"testing"
)

func TestSort(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []string
		edges   map[string][]string
		want    []string
		wantErr bool
	}{
		{
			name:  "empty",
			nodes: []string{},
			edges: nil,
			want:  []string{},
		},
		{
			name:  "single node no edges",
			nodes: []string{"A"},
			edges: nil,
			want:  []string{"A"},
		},
		{
			name:  "linear chain A->B->C",
			nodes: []string{"A", "B", "C"},
			edges: map[string][]string{
				"B": {"A"},
				"C": {"B"},
			},
			want: []string{"A", "B", "C"},
		},
		{
			name:  "linear chain unsorted input",
			nodes: []string{"C", "A", "B"},
			edges: map[string][]string{
				"B": {"A"},
				"C": {"B"},
			},
			want: []string{"A", "B", "C"},
		},
		{
			name:  "diamond A->B,A->C,B->D,C->D",
			nodes: []string{"A", "B", "C", "D"},
			edges: map[string][]string{
				"B": {"A"},
				"C": {"A"},
				"D": {"B", "C"},
			},
			want: []string{"A", "B", "C", "D"},
		},
		{
			name:  "two independent roots alphabetical",
			nodes: []string{"B", "A"},
			edges: nil,
			want:  []string{"A", "B"},
		},
		{
			name:  "many independent nodes alphabetical",
			nodes: []string{"d", "b", "a", "c"},
			edges: nil,
			want:  []string{"a", "b", "c", "d"},
		},
		{
			name:  "mixed roots and chain",
			nodes: []string{"X", "A", "B", "C"},
			edges: map[string][]string{
				"B": {"A"},
				"C": {"B"},
			},
			want: []string{"A", "B", "C", "X"},
		},
		{
			name:  "duplicate edges treated once",
			nodes: []string{"A", "B"},
			edges: map[string][]string{
				"B": {"A", "A"},
			},
			want: []string{"A", "B"},
		},
		{
			name:  "edge to unknown node ignored",
			nodes: []string{"A"},
			edges: map[string][]string{
				"A": {"ghost"},
			},
			want: []string{"A"},
		},
		{
			name:  "cycle A->B->C->A",
			nodes: []string{"A", "B", "C"},
			edges: map[string][]string{
				"A": {"C"},
				"B": {"A"},
				"C": {"B"},
			},
			wantErr: true,
		},
		{
			name:  "self-loop A->A",
			nodes: []string{"A"},
			edges: map[string][]string{
				"A": {"A"},
			},
			wantErr: true,
		},
		{
			name:  "cycle with extra acyclic prefix",
			nodes: []string{"root", "A", "B"},
			edges: map[string][]string{
				"A": {"B", "root"},
				"B": {"A"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, cerr := Sort(tt.nodes, tt.edges)
			if tt.wantErr {
				if cerr == nil {
					t.Fatalf("expected CycleError, got nil; result=%v", got)
				}
				if len(cerr.Path) < 2 {
					t.Fatalf("CycleError.Path too short: %v", cerr.Path)
				}
				if cerr.Path[0] != cerr.Path[len(cerr.Path)-1] {
					t.Fatalf("CycleError.Path should start and end with same node: %v", cerr.Path)
				}
				if cerr.Error() == "" {
					t.Fatalf("Error() returned empty string")
				}
				return
			}
			if cerr != nil {
				t.Fatalf("unexpected CycleError: %v", cerr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Sort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortDeterministic(t *testing.T) {
	nodes := []string{"D", "C", "B", "A", "E"}
	edges := map[string][]string{
		"E": {"A", "B"},
		"D": {"A"},
	}
	first, err := Sort(nodes, edges)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 50; i++ {
		got, err := Sort(nodes, edges)
		if err != nil {
			t.Fatalf("unexpected error on iter %d: %v", i, err)
		}
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("non-deterministic output: %v vs %v", got, first)
		}
	}
}

func TestCycleErrorMessage(t *testing.T) {
	e := &CycleError{Path: []string{"a", "b", "c", "a"}}
	want := "cycle detected: a → b → c → a"
	if e.Error() != want {
		t.Fatalf("Error() = %q, want %q", e.Error(), want)
	}
}

func TestSortCycleSelfLoopPath(t *testing.T) {
	_, cerr := Sort([]string{"A"}, map[string][]string{"A": {"A"}})
	if cerr == nil {
		t.Fatal("expected CycleError")
	}
	want := []string{"A", "A"}
	if !reflect.DeepEqual(cerr.Path, want) {
		t.Fatalf("Path = %v, want %v", cerr.Path, want)
	}
}

func TestSortCycleThreeNodePath(t *testing.T) {
	_, cerr := Sort([]string{"A", "B", "C"}, map[string][]string{
		"A": {"C"},
		"B": {"A"},
		"C": {"B"},
	})
	if cerr == nil {
		t.Fatal("expected CycleError")
	}
	if len(cerr.Path) != 4 {
		t.Fatalf("expected 4 elements (3 nodes + repeat), got %v", cerr.Path)
	}
	seen := map[string]bool{}
	for _, n := range cerr.Path[:3] {
		seen[n] = true
	}
	for _, n := range []string{"A", "B", "C"} {
		if !seen[n] {
			t.Fatalf("expected node %s in cycle, got %v", n, cerr.Path)
		}
	}
}
