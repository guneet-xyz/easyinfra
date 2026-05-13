package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

func writeTestReplicaState(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, replicasFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write replicas.json: %v", err)
	}
}

func TestRecovery(t *testing.T) {
	root := t.TempDir()
	ts := "2025-05-13_120000"
	dir := filepath.Join(root, ts)
	writeTestReplicaState(t, dir, `{"deployments":{"caddy/caddy":1,"walls/walls":2}}`)

	fake := &exec.FakeRunner{Default: exec.FakeResponse{Stdout: "ok"}}

	if err := Recover(context.Background(), root, ts, fake); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	wantSubstrs := []string{
		"scale deployment caddy -n caddy --replicas=1",
		"scale deployment walls -n walls --replicas=2",
	}
	for _, want := range wantSubstrs {
		found := false
		for _, c := range fake.Calls {
			joined := strings.Join(append([]string{c.Name}, c.Args...), " ")
			if strings.Contains(joined, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing call containing %q; calls: %+v", want, fake.Calls)
		}
	}
}

func TestRecoveryWithAppFilter(t *testing.T) {
	root := t.TempDir()
	ts := "2025-05-13_120000"
	dir := filepath.Join(root, ts)
	writeTestReplicaState(t, dir, `{"deployments":{"caddy/caddy":1,"walls/walls":2}}`)

	fake := &exec.FakeRunner{Default: exec.FakeResponse{Stdout: "ok"}}

	if err := Recover(context.Background(), root, ts, fake, "caddy"); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	for _, c := range fake.Calls {
		joined := strings.Join(append([]string{c.Name}, c.Args...), " ")
		if strings.Contains(joined, "-n walls") {
			t.Errorf("walls scaled despite filter; call: %s", joined)
		}
	}
}

func TestRecoveryClearsStateMarkerOnSuccess(t *testing.T) {
	root := t.TempDir()
	ts := "2025-05-13_120000"
	dir := filepath.Join(root, ts)
	writeTestReplicaState(t, dir, `{"deployments":{"caddy/caddy":1}}`)
	if err := writeStateFile(dir, stateScaleUpFail); err != nil {
		t.Fatalf("seed state file: %v", err)
	}

	fake := &exec.FakeRunner{Default: exec.FakeResponse{Stdout: "ok"}}
	if err := Recover(context.Background(), root, ts, fake); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, stateFileName)); !os.IsNotExist(err) {
		t.Errorf("state marker not removed: err=%v", err)
	}
}

func TestRecoveryAfterScaleUpFail(t *testing.T) {
	mgr, fake := newTestManager(t)
	fake.Responses["kubectl scale deployment alpha-deploy -n alpha --replicas=2 --timeout=120s"] = exec.FakeResponse{
		Err: errors.New("scale failed"),
	}

	apps := []config.AppConfig{
		{Name: "alpha", Namespace: "alpha", PVCs: []string{"alpha-data"}},
	}
	ts, err := mgr.Run(context.Background(), apps)
	if err == nil {
		t.Fatalf("expected error from scale-up failure")
	}

	stateFile := filepath.Join(mgr.Cfg.LocalDir, ts, stateFileName)
	data, readErr := os.ReadFile(stateFile)
	if readErr != nil {
		t.Fatalf("expected state file %s: %v", stateFile, readErr)
	}
	if string(data) != stateScaleUpFail {
		t.Errorf("state file content = %q, want %q", string(data), stateScaleUpFail)
	}

	repFile := filepath.Join(mgr.Cfg.LocalDir, ts, replicasFileName)
	if _, err := os.Stat(repFile); err != nil {
		t.Errorf("expected replicas.json at %s: %v", repFile, err)
	}
}
