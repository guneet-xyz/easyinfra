package deps

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCheck_MissingLock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Chart.yaml"), `apiVersion: v2
name: walls
dependencies:
  - name: postgres
    version: "0.1.0"
    repository: "file://./postgres"
`)
	writeFile(t, filepath.Join(dir, "postgres", "Chart.yaml"), "name: postgres\n")

	issues, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 || issues[0].Kind != "missing-lock" {
		t.Fatalf("expected missing-lock issue, got %+v", issues)
	}
}

func TestCheck_MatchingLock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Chart.yaml"), `apiVersion: v2
name: walls
dependencies:
  - name: postgres
    version: "0.1.0"
    repository: "file://./postgres"
`)
	writeFile(t, filepath.Join(dir, "Chart.lock"), `dependencies:
  - name: postgres
    repository: "file://./postgres"
    version: 0.1.0
generated: "2024-01-01T00:00:00.000000000Z"
digest: sha256:abc123
`)
	writeFile(t, filepath.Join(dir, "postgres", "Chart.yaml"), "name: postgres\n")

	issues, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
}

func TestCheck_LockStale(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Chart.yaml"), `apiVersion: v2
name: walls
dependencies:
  - name: postgres
    version: "0.1.0"
    repository: "file://./postgres"
  - name: redis
    version: "0.1.0"
    repository: "file://./redis"
`)
	writeFile(t, filepath.Join(dir, "Chart.lock"), `dependencies:
  - name: postgres
    repository: "file://./postgres"
    version: 0.1.0
generated: "2024-01-01T00:00:00.000000000Z"
digest: sha256:abc123
`)
	writeFile(t, filepath.Join(dir, "postgres", "Chart.yaml"), "name: postgres\n")
	writeFile(t, filepath.Join(dir, "redis", "Chart.yaml"), "name: redis\n")

	issues, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	found := false
	for _, i := range issues {
		if i.Kind == "lock-stale" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected lock-stale issue, got %+v", issues)
	}
}

func TestCheck_FileDepMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Chart.yaml"), `apiVersion: v2
name: walls
dependencies:
  - name: postgres
    version: "0.1.0"
    repository: "file://./does-not-exist"
`)
	writeFile(t, filepath.Join(dir, "Chart.lock"), `dependencies:
  - name: postgres
    repository: "file://./does-not-exist"
    version: 0.1.0
generated: "2024-01-01T00:00:00.000000000Z"
digest: sha256:abc123
`)

	issues, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	found := false
	for _, i := range issues {
		if i.Kind == "file-dep-missing" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected file-dep-missing issue, got %+v", issues)
	}
}

func TestUpdate_CallsHelm(t *testing.T) {
	runner := &exec.FakeRunner{}
	if err := Update(context.Background(), runner, "/charts/walls"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}
	c := runner.Calls[0]
	if c.Name != "helm" {
		t.Fatalf("expected helm, got %s", c.Name)
	}
	want := []string{"dependency", "update", "/charts/walls"}
	if len(c.Args) != len(want) {
		t.Fatalf("args mismatch: got %v want %v", c.Args, want)
	}
	for i := range want {
		if c.Args[i] != want[i] {
			t.Fatalf("arg[%d]: got %s want %s", i, c.Args[i], want[i])
		}
	}
}
