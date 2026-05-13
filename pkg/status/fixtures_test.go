package status

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/guneet-xyz/easyinfra/pkg/history"
)

func TestFixturesParse(t *testing.T) {
	repoRoot := filepath.Join("..", "..")

	statusDir := filepath.Join(repoRoot, "testdata", "helm", "status")
	statusFiles, err := filepath.Glob(filepath.Join(statusDir, "*.json"))
	if err != nil {
		t.Fatalf("glob status fixtures: %v", err)
	}
	if len(statusFiles) == 0 {
		t.Fatalf("no status fixtures found in %s", statusDir)
	}
	for _, f := range statusFiles {
		t.Run("status/"+filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			runner := &exec.FakeRunner{
				Default: exec.FakeResponse{Stdout: string(data)},
			}
			rel, err := Status(context.Background(), runner, "fixture", "fixture")
			if err != nil {
				t.Fatalf("Status parse %s: %v", f, err)
			}
			if rel == nil {
				t.Fatalf("Status returned nil release for %s", f)
			}
			if rel.Name == "" {
				t.Errorf("%s: empty release name", f)
			}
		})
	}

	historyDir := filepath.Join(repoRoot, "testdata", "helm", "history")
	historyFiles, err := filepath.Glob(filepath.Join(historyDir, "*.json"))
	if err != nil {
		t.Fatalf("glob history fixtures: %v", err)
	}
	if len(historyFiles) == 0 {
		t.Fatalf("no history fixtures found in %s", historyDir)
	}
	for _, f := range historyFiles {
		t.Run("history/"+filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			runner := &exec.FakeRunner{
				Default: exec.FakeResponse{Stdout: string(data)},
			}
			revs, err := history.History(context.Background(), runner, "fixture", "fixture", 10)
			if err != nil {
				t.Fatalf("History parse %s: %v", f, err)
			}
			if len(revs) == 0 {
				t.Errorf("%s: parsed zero revisions", f)
			}
		})
	}
}
