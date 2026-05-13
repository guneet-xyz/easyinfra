package history

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

func TestHistory(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "history.json"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	runner := &exec.FakeRunner{
		Default: exec.FakeResponse{Stdout: string(data)},
	}

	revs, err := History(context.Background(), runner, "caddy", "caddy", 10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(revs) != 5 {
		t.Fatalf("len(revs) = %d, want 5", len(revs))
	}
	if revs[0].Revision != 5 {
		t.Errorf("revs[0].Revision = %d, want 5 (newest first)", revs[0].Revision)
	}
	if revs[len(revs)-1].Revision != 1 {
		t.Errorf("revs[last].Revision = %d, want 1", revs[len(revs)-1].Revision)
	}
	if revs[0].Status != "deployed" {
		t.Errorf("revs[0].Status = %q, want %q", revs[0].Status, "deployed")
	}
	if revs[0].Chart != "caddy-0.1.0" {
		t.Errorf("revs[0].Chart = %q, want %q", revs[0].Chart, "caddy-0.1.0")
	}
	if revs[0].AppVersion != "2.7.0" {
		t.Errorf("revs[0].AppVersion = %q, want %q", revs[0].AppVersion, "2.7.0")
	}
	if revs[0].Updated.IsZero() {
		t.Errorf("revs[0].Updated is zero")
	}
}

func TestHistoryNotFound(t *testing.T) {
	runner := &exec.FakeRunner{
		Default: exec.FakeResponse{
			Stderr: "Error: release: not found",
			Err:    errors.New("exit status 1"),
		},
	}

	revs, err := History(context.Background(), runner, "missing", "default", 10)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if revs != nil {
		t.Errorf("revs = %+v, want nil", revs)
	}
}
