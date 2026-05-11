package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guneet/easyinfra/pkg/config"
	"github.com/guneet/easyinfra/pkg/exec"
	"github.com/guneet/easyinfra/pkg/k8s"
)

func newTestManager(t *testing.T) (*Manager, *exec.FakeRunner) {
	t.Helper()
	fake := &exec.FakeRunner{
		Default: exec.FakeResponse{Stdout: "ok"},
		Responses: map[string]exec.FakeResponse{
			"kubectl get deployments -n alpha -o jsonpath={.items[*].metadata.name}": {Stdout: "alpha-deploy"},
			"kubectl get deployments -n beta -o jsonpath={.items[*].metadata.name}":  {Stdout: "beta-deploy"},
			"kubectl get deployment alpha-deploy -n alpha -o jsonpath={.spec.replicas}": {Stdout: "2"},
			"kubectl get deployment beta-deploy -n beta -o jsonpath={.spec.replicas}":   {Stdout: "1"},
			"kubectl get pvc alpha-data -n alpha -o jsonpath={.spec.volumeName}": {Stdout: "pv-alpha"},
			"kubectl get pvc beta-data -n beta -o jsonpath={.spec.volumeName}":   {Stdout: "pv-beta"},
			"kubectl get pv pv-alpha -o jsonpath={.spec.local.path}": {Stdout: "/var/lib/alpha"},
			"kubectl get pv pv-beta -o jsonpath={.spec.local.path}":  {Stdout: "/var/lib/beta"},
		},
	}
	k8sClient := &k8s.Client{Runner: fake}
	mgr := &Manager{
		Runner: fake,
		K8s:    k8sClient,
		Cfg: config.BackupConfig{
			RemoteHost: "test-host",
			RemoteTmp:  "/tmp/backups",
			LocalDir:   t.TempDir(),
		},
	}
	return mgr, fake
}

func callKey(c exec.FakeCall) string {
	return strings.Join(append([]string{c.Name}, c.Args...), " ")
}

func hasCallContaining(calls []exec.FakeCall, substr string) bool {
	for _, c := range calls {
		if strings.Contains(callKey(c), substr) {
			return true
		}
	}
	return false
}

func TestBackupOrchestration(t *testing.T) {
	mgr, fake := newTestManager(t)
	apps := []config.AppConfig{
		{Name: "alpha", Namespace: "alpha", PVCs: []string{"alpha-data"}},
	}
	_, err := mgr.Run(context.Background(), apps)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expectations := []string{
		"kubectl get deployments -n alpha",
		"kubectl get deployment alpha-deploy -n alpha",
		"kubectl scale deployment alpha-deploy -n alpha --replicas=0",
		"kubectl wait --for=delete pod --all -n alpha",
		"ssh test-host tar czf /tmp/backups/",
		"kubectl scale deployment alpha-deploy -n alpha --replicas=2",
		"scp -r test-host:/tmp/backups/",
		"ssh test-host rm -rf /tmp/backups/",
	}
	for _, exp := range expectations {
		if !hasCallContaining(fake.Calls, exp) {
			t.Errorf("missing expected call containing: %q\nactual calls:\n%s", exp, dumpCalls(fake.Calls))
		}
	}
}

func TestBackupSkipsNoPVCs(t *testing.T) {
	mgr, fake := newTestManager(t)
	apps := []config.AppConfig{
		{Name: "empty", Namespace: "empty"},
	}
	_, err := mgr.Run(context.Background(), apps)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	for _, c := range fake.Calls {
		if c.Name == "kubectl" && hasArg(c.Args, "empty") {
			t.Errorf("expected no kubectl calls for empty namespace, got: %v", c.Args)
		}
	}
}

func hasArg(args []string, target string) bool {
	for _, a := range args {
		if a == target {
			return true
		}
	}
	return false
}

func TestBackupPartialFailure(t *testing.T) {
	mgr, fake := newTestManager(t)
	apps := []config.AppConfig{
		{Name: "alpha", Namespace: "alpha", PVCs: []string{"alpha-data"}},
		{Name: "beta", Namespace: "beta", PVCs: []string{"beta-data"}},
	}
	wrapped := &failingRunner{inner: fake, failSubstr: "tar czf", failOnlyContains: "alpha-data"}
	mgr.Runner = wrapped
	mgr.K8s = &k8s.Client{Runner: wrapped}

	_, err := mgr.Run(context.Background(), apps)
	if err == nil {
		t.Fatal("expected error from partial failure, got nil")
	}
	if !strings.Contains(err.Error(), "alpha") {
		t.Errorf("expected error to mention failed app 'alpha', got: %v", err)
	}
	if !hasCallContaining(fake.Calls, "kubectl get deployments -n beta") {
		t.Errorf("expected beta backup to still be attempted")
	}
	if !hasCallContaining(fake.Calls, "kubectl scale deployment alpha-deploy -n alpha --replicas=2") {
		t.Errorf("expected replicas to be restored even on tar failure")
	}
}

type failingRunner struct {
	inner            *exec.FakeRunner
	failSubstr       string
	failOnlyContains string
}

func (f *failingRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	if strings.Contains(key, f.failSubstr) && strings.Contains(key, f.failOnlyContains) {
		f.inner.Calls = append(f.inner.Calls, exec.FakeCall{Name: name, Args: args})
		return "", "", errors.New("simulated tar failure")
	}
	return f.inner.Run(ctx, name, args...)
}

func (f *failingRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	return f.inner.RunInteractive(ctx, name, args...)
}

func TestRestoreLatest(t *testing.T) {
	mgr, fake := newTestManager(t)
	for _, d := range []string{"2026-05-10_120000", "2026-05-11_120000"} {
		if err := os.MkdirAll(filepath.Join(mgr.Cfg.LocalDir, d), 0755); err != nil {
			t.Fatal(err)
		}
		// Create dummy tar for the pvc.
		f, err := os.Create(filepath.Join(mgr.Cfg.LocalDir, d, "alpha-data.tar.gz"))
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	apps := []config.AppConfig{
		{Name: "alpha", Namespace: "alpha", PVCs: []string{"alpha-data"}},
	}
	if err := mgr.Restore(context.Background(), apps, ""); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	if !hasCallContaining(fake.Calls, "/tmp/backups/2026-05-11_120000") {
		t.Errorf("expected latest timestamp 2026-05-11_120000 to be used\ncalls:\n%s", dumpCalls(fake.Calls))
	}
}

func TestRestoreMissingTimestamp(t *testing.T) {
	mgr, _ := newTestManager(t)
	apps := []config.AppConfig{
		{Name: "alpha", Namespace: "alpha", PVCs: []string{"alpha-data"}},
	}
	err := mgr.Restore(context.Background(), apps, "nonexistent-ts")
	if err == nil {
		t.Fatal("expected error for missing timestamp, got nil")
	}
	expectedPath := filepath.Join(mgr.Cfg.LocalDir, "nonexistent-ts")
	if !strings.Contains(err.Error(), expectedPath) {
		t.Errorf("expected error to contain path %q, got: %v", expectedPath, err)
	}
}

func TestLatestTimestamp(t *testing.T) {
	mgr, _ := newTestManager(t)
	for _, d := range []string{"2026-01-01_000000", "2026-06-15_120000", "2026-03-10_080000"} {
		if err := os.MkdirAll(filepath.Join(mgr.Cfg.LocalDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	ts, err := mgr.LatestTimestamp()
	if err != nil {
		t.Fatalf("LatestTimestamp failed: %v", err)
	}
	if ts != "2026-06-15_120000" {
		t.Errorf("expected 2026-06-15_120000, got %s", ts)
	}
}

func TestRestoreHappyPath(t *testing.T) {
	mgr, fake := newTestManager(t)
	mgr.Cfg.RemoteUser = "deploy"
	ts := "2026-05-11_120000"
	if err := os.MkdirAll(filepath.Join(mgr.Cfg.LocalDir, ts), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(mgr.Cfg.LocalDir, ts, "alpha-data.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	apps := []config.AppConfig{
		{Name: "alpha", Namespace: "alpha", PVCs: []string{"alpha-data"}},
		{Name: "skip", Namespace: "skip"},
	}
	if err := mgr.Restore(context.Background(), apps, ts); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	expects := []string{
		"ssh deploy@test-host mkdir -p /tmp/backups/2026-05-11_120000",
		"scp",
		"ssh deploy@test-host rm -rf",
		"tar xzf /tmp/backups/2026-05-11_120000/alpha-data.tar.gz",
	}
	for _, e := range expects {
		if !hasCallContaining(fake.Calls, e) {
			t.Errorf("missing call containing %q\ncalls:\n%s", e, dumpCalls(fake.Calls))
		}
	}
}

func TestRestoreScpFailure(t *testing.T) {
	mgr, fake := newTestManager(t)
	ts := "2026-05-11_120000"
	if err := os.MkdirAll(filepath.Join(mgr.Cfg.LocalDir, ts), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(mgr.Cfg.LocalDir, ts, "alpha-data.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	wrapped := &failingRunner{inner: fake, failSubstr: "scp", failOnlyContains: "alpha-data.tar.gz"}
	mgr.Runner = wrapped
	mgr.K8s = &k8s.Client{Runner: wrapped}

	apps := []config.AppConfig{
		{Name: "alpha", Namespace: "alpha", PVCs: []string{"alpha-data"}},
	}
	err = mgr.Restore(context.Background(), apps, ts)
	if err == nil {
		t.Fatal("expected error from scp failure")
	}
	if !strings.Contains(err.Error(), "scp") {
		t.Errorf("expected scp error, got: %v", err)
	}
	if !hasCallContaining(fake.Calls, "kubectl scale deployment alpha-deploy -n alpha --replicas=2") {
		t.Error("expected replicas to be restored even after scp failure")
	}
}

func TestBackupScpFailure(t *testing.T) {
	mgr, fake := newTestManager(t)
	wrapped := &failingRunner{inner: fake, failSubstr: "scp", failOnlyContains: "-r"}
	mgr.Runner = wrapped
	mgr.K8s = &k8s.Client{Runner: wrapped}
	apps := []config.AppConfig{
		{Name: "alpha", Namespace: "alpha", PVCs: []string{"alpha-data"}},
	}
	_, err := mgr.Run(context.Background(), apps)
	if err == nil {
		t.Fatal("expected error from scp failure")
	}
	if !strings.Contains(err.Error(), "scp from remote") {
		t.Errorf("expected scp from remote error, got: %v", err)
	}
}

func TestLatestTimestampEmpty(t *testing.T) {
	mgr, _ := newTestManager(t)
	_, err := mgr.LatestTimestamp()
	if err == nil {
		t.Fatal("expected error for empty backup dir")
	}
}

func dumpCalls(calls []exec.FakeCall) string {
	var sb strings.Builder
	for i, c := range calls {
		sb.WriteString("  [")
		sb.WriteString(itoa(i))
		sb.WriteString("] ")
		sb.WriteString(callKey(c))
		sb.WriteString("\n")
	}
	return sb.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
