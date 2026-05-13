package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIncludeGlobSorted(t *testing.T) {
	parts, err := IncludesResolve("testdata/includes/glob", []string{"*.yaml"})
	if err != nil {
		t.Fatalf("IncludesResolve: %v", err)
	}
	if len(parts) != 3 {
		t.Fatalf("want 3 partials, got %d", len(parts))
	}
	wantOrder := []string{"a.yaml", "b.yaml", "c.yaml"}
	for i, p := range parts {
		if !strings.HasSuffix(p.Source, wantOrder[i]) {
			t.Errorf("partial %d: source=%q, want suffix %q", i, p.Source, wantOrder[i])
		}
		if !filepath.IsAbs(p.Source) {
			t.Errorf("partial %d: source %q is not absolute", i, p.Source)
		}
	}
	wantApps := []string{"alpha", "beta", "gamma"}
	for i, p := range parts {
		if len(p.Apps) != 1 || p.Apps[0].Name != wantApps[i] {
			t.Errorf("partial %d: want app %q, got %+v", i, wantApps[i], p.Apps)
		}
	}
}

func TestIncludeCycleDetected(t *testing.T) {
	_, err := IncludesResolve("testdata/includes/cycle", []string{"a.yaml"})
	if err == nil {
		t.Fatal("want cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("want cycle error, got %v", err)
	}
}

func TestIncludeMissingFile(t *testing.T) {
	_, err := IncludesResolve("testdata/includes/glob", []string{"does-not-exist.yaml"})
	if err == nil {
		t.Fatal("want missing-file error, got nil")
	}
	if !strings.Contains(err.Error(), "no files match") {
		t.Errorf("want no-match error, got %v", err)
	}
}

func TestIncludeDuplicateApp(t *testing.T) {
	_, err := IncludesResolve("testdata/includes/dup", []string{"one.yaml", "two.yaml"})
	if err == nil {
		t.Fatal("want duplicate-app error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate app") || !strings.Contains(err.Error(), "shared") {
		t.Errorf("want duplicate-app error mentioning \"shared\", got %v", err)
	}
}
