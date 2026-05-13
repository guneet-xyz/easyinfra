package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func seedBackups(t *testing.T, dir string, timestamps []string) {
	t.Helper()
	for _, ts := range timestamps {
		p := filepath.Join(dir, ts)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(filepath.Join(p, "marker"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write marker: %v", err)
		}
	}
}

func TestPruneKeepN(t *testing.T) {
	dir := t.TempDir()
	timestamps := []string{
		"2026-01-01_120000",
		"2026-01-02_120000",
		"2026-01-03_120000",
		"2026-01-04_120000",
		"2026-01-05_120000",
	}
	seedBackups(t, dir, timestamps)

	deleted, err := Prune(dir, PrunePolicy{KeepN: 2})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(deleted) != 3 {
		t.Fatalf("expected 3 deleted, got %d: %v", len(deleted), deleted)
	}

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(entries))
	}
	if entries[0].Timestamp != "2026-01-05_120000" || entries[1].Timestamp != "2026-01-04_120000" {
		t.Fatalf("wrong remaining entries: %+v", entries)
	}
}

func TestPruneDryRun(t *testing.T) {
	dir := t.TempDir()
	timestamps := []string{
		"2026-01-01_120000",
		"2026-01-02_120000",
		"2026-01-03_120000",
		"2026-01-04_120000",
		"2026-01-05_120000",
	}
	seedBackups(t, dir, timestamps)

	candidates, err := Prune(dir, PrunePolicy{KeepN: 2, DryRun: true})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d: %v", len(candidates), candidates)
	}

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("dry-run should not delete; got %d entries", len(entries))
	}
}

func TestPruneOlderThan(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	old := now.Add(-72 * time.Hour).Format(timestampLayout)
	older := now.Add(-200 * time.Hour).Format(timestampLayout)
	recent := now.Add(-1 * time.Hour).Format(timestampLayout)
	seedBackups(t, dir, []string{old, older, recent})

	deleted, err := Prune(dir, PrunePolicy{OlderThan: 48 * time.Hour})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deleted, got %d: %v", len(deleted), deleted)
	}

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Timestamp != recent {
		t.Fatalf("expected only %s remaining, got %+v", recent, entries)
	}
}

func TestPruneEmpty(t *testing.T) {
	dir := t.TempDir()
	deleted, err := Prune(dir, PrunePolicy{KeepN: 2})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("expected 0 deleted, got %d", len(deleted))
	}
}
