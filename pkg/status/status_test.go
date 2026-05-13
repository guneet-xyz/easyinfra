package status

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

func TestStatusParse(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "deployed.json"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	runner := &exec.FakeRunner{
		Default: exec.FakeResponse{Stdout: string(data)},
	}

	rel, err := Status(context.Background(), runner, "caddy", "caddy")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if rel.Revision != 3 {
		t.Errorf("Revision = %d, want 3", rel.Revision)
	}
	if rel.Status != "deployed" {
		t.Errorf("Status = %q, want %q", rel.Status, "deployed")
	}
	if rel.Name != "caddy" {
		t.Errorf("Name = %q, want %q", rel.Name, "caddy")
	}
	if rel.Namespace != "caddy" {
		t.Errorf("Namespace = %q, want %q", rel.Namespace, "caddy")
	}
	if rel.Chart.Name != "caddy" || rel.Chart.Version != "0.1.0" {
		t.Errorf("Chart = %+v, want {caddy 0.1.0}", rel.Chart)
	}
	if rel.FirstDeployed.IsZero() || rel.LastDeployed.IsZero() {
		t.Errorf("timestamps not parsed: first=%v last=%v", rel.FirstDeployed, rel.LastDeployed)
	}
}

func TestStatusNotFound(t *testing.T) {
	runner := &exec.FakeRunner{
		Default: exec.FakeResponse{
			Stderr: "Error: release: not found",
			Err:    errors.New("exit status 1"),
		},
	}

	rel, err := Status(context.Background(), runner, "missing", "default")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if rel != nil {
		t.Errorf("rel = %+v, want nil", rel)
	}
}
