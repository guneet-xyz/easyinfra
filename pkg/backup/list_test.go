package backup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestList_NewestFirstAndReplicasDetection(t *testing.T) {
	root := t.TempDir()

	older := filepath.Join(root, "2026-01-01_120000")
	if err := os.MkdirAll(older, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(older, "caddy.tar"), make([]byte, 100))
	writeFile(t, filepath.Join(older, "walls.tar"), make([]byte, 200))
	writeFile(t, filepath.Join(older, "replicas.json"), []byte("{}"))

	newer := filepath.Join(root, "2026-01-02_120000")
	if err := os.MkdirAll(newer, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(newer, "caddy.tar"), make([]byte, 50))

	writeFile(t, filepath.Join(root, "stray.txt"), []byte("nope"))
	if err := os.MkdirAll(filepath.Join(root, "not-a-timestamp"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := List(root)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(got), got)
	}

	if got[0].Timestamp != "2026-01-02_120000" {
		t.Errorf("expected newest first, got %s", got[0].Timestamp)
	}
	if got[1].Timestamp != "2026-01-01_120000" {
		t.Errorf("expected older second, got %s", got[1].Timestamp)
	}

	if !reflect.DeepEqual(got[0].Apps, []string{"caddy"}) {
		t.Errorf("newer apps = %v, want [caddy]", got[0].Apps)
	}
	if got[0].SizeBytes != 50 {
		t.Errorf("newer size = %d, want 50", got[0].SizeBytes)
	}
	if got[0].HasReplicas {
		t.Error("newer should not have replicas")
	}

	if !reflect.DeepEqual(got[1].Apps, []string{"caddy", "walls"}) {
		t.Errorf("older apps = %v, want [caddy walls]", got[1].Apps)
	}
	if got[1].SizeBytes != 100+200+2 {
		t.Errorf("older size = %d, want %d", got[1].SizeBytes, 100+200+2)
	}
	if !got[1].HasReplicas {
		t.Error("older should have replicas")
	}
}

func TestList_MissingDirReturnsNil(t *testing.T) {
	got, err := List(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("expected no error for missing dir, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil entries, got %+v", got)
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
