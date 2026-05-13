package backup

import (
	"bytes"
	"context"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/config"
)

type recordingRunner struct {
	calls [][]string
}

func (r *recordingRunner) Run(_ context.Context, cmd string, args ...string) (string, string, error) {
	r.calls = append(r.calls, append([]string{cmd}, args...))
	return "", "", nil
}

func (r *recordingRunner) RunInteractive(_ context.Context, cmd string, args ...string) error {
	r.calls = append(r.calls, append([]string{cmd}, args...))
	return nil
}

func TestPlan(t *testing.T) {
	cfg := &config.InfraConfigV2{
		Backup: config.BackupConfigV2{
			RemoteHost: "host",
			RemoteTmp:  "/tmp/bk",
			LocalDir:   "/var/backups",
		},
		Apps: []config.AppConfigV2{
			{Name: "a", Namespace: "ns-a", PVCs: []string{"pvc1", "pvc2"}},
			{Name: "b", Namespace: "ns-b"},
			{Name: "c", Namespace: "ns-c", PVCs: []string{"pvc3"}},
		},
	}

	op := Plan(cfg, nil)
	if op == nil {
		t.Fatal("Plan returned nil")
	}
	if op.Timestamp == "" {
		t.Error("Plan returned empty timestamp")
	}
	if len(op.Apps) != 2 {
		t.Fatalf("expected 2 apps in plan (b skipped), got %d", len(op.Apps))
	}
	if op.Apps[0].AppName != "a" || op.Apps[0].Namespace != "ns-a" {
		t.Errorf("apps[0] = %+v, want a/ns-a", op.Apps[0])
	}
	if len(op.Apps[0].PVCs) != 2 || op.Apps[0].PVCs[0] != "pvc1" {
		t.Errorf("apps[0].PVCs = %v", op.Apps[0].PVCs)
	}
	if op.Apps[1].AppName != "c" {
		t.Errorf("apps[1] = %+v, want c", op.Apps[1])
	}
	wantRemote := "/tmp/bk/" + op.Timestamp
	if op.Apps[0].RemotePath != wantRemote {
		t.Errorf("RemotePath = %q, want %q", op.Apps[0].RemotePath, wantRemote)
	}

	op2 := Plan(cfg, []string{"c"})
	if len(op2.Apps) != 1 || op2.Apps[0].AppName != "c" {
		t.Errorf("filtered plan = %+v, want only c", op2.Apps)
	}
}

func TestExecuteDryRun(t *testing.T) {
	cfg := &config.InfraConfigV2{
		Backup: config.BackupConfigV2{RemoteHost: "h", RemoteTmp: "/tmp", LocalDir: "/d"},
		Apps: []config.AppConfigV2{
			{Name: "a", Namespace: "ns", PVCs: []string{"p1"}},
		},
	}
	op := Plan(cfg, nil)
	rr := &recordingRunner{}
	var buf bytes.Buffer
	if err := ExecuteWith(context.Background(), op, rr, true, &buf, "", ""); err != nil {
		t.Fatalf("dry-run Execute failed: %v", err)
	}
	if len(rr.calls) != 0 {
		t.Errorf("expected zero runner calls in dry-run, got %d: %v", len(rr.calls), rr.calls)
	}
	if buf.Len() == 0 {
		t.Error("expected dry-run to produce output, got none")
	}
}
