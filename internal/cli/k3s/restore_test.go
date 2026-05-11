package k3s

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guneet/easyinfra/pkg/exec"
	"github.com/stretchr/testify/require"
)

// restoreFixture creates a fixture directory by copying testdata/infra, rewriting
// infra.yaml to use an absolute LocalDir under the fixture, and creating the
// given timestamp directories each populated with fake tar files for alpha-data.
func restoreFixture(t *testing.T, timestamps ...string) (cfgPath, localDir string) {
	t.Helper()
	fixture := makeFixture(t)

	localDir = filepath.Join(fixture, "backups")
	require.NoError(t, os.MkdirAll(localDir, 0o755))

	// Rewrite infra.yaml so LocalDir is absolute (testdata/backups is relative).
	cfgPath = filepath.Join(fixture, "infra.yaml")
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	rewritten := strings.Replace(
		string(data),
		"localDir: testdata/backups",
		"localDir: "+localDir,
		1,
	)
	require.NoError(t, os.WriteFile(cfgPath, []byte(rewritten), 0o644))

	for _, ts := range timestamps {
		tsDir := filepath.Join(localDir, ts)
		require.NoError(t, os.MkdirAll(tsDir, 0o755))
		// Fake tar for alpha-data (only alpha has PVCs in testdata).
		require.NoError(t, os.WriteFile(
			filepath.Join(tsDir, "alpha-data.tar.gz"),
			[]byte("fake tar"),
			0o644,
		))
	}
	return cfgPath, localDir
}

func defaultRestoreFake() *exec.FakeRunner {
	return &exec.FakeRunner{
		Responses: map[string]exec.FakeResponse{
			"kubectl config current-context": {Stdout: "test-ctx"},
			"kubectl get deployments -n alpha -o jsonpath={.items[*].metadata.name}":      {Stdout: "alpha-deploy"},
			"kubectl get deployment alpha-deploy -n alpha -o jsonpath={.spec.replicas}":   {Stdout: "2"},
			"kubectl get pvc alpha-data -n alpha -o jsonpath={.spec.volumeName}":          {Stdout: "pv-alpha"},
			"kubectl get pv pv-alpha -o jsonpath={.spec.local.path}":                      {Stdout: "/var/lib/alpha"},
		},
		Default: exec.FakeResponse{},
	}
}

func TestRestoreExplicitTimestamp(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-01-01_120000", "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"k3s", "restore",
		"--config", cfgPath,
		"--timestamp", "2026-01-01_120000",
		"--yes",
	})

	require.NoError(t, cmd.Execute())

	stdout := out.String()
	require.Contains(t, stdout, "Restored 1 apps from 2026-01-01_120000")

	// Sanity: the explicit (older) timestamp was used, not the latest.
	require.NotContains(t, stdout, "2026-05-11_120000")

	// Verify backup.Manager.Restore actually ran ssh mkdir against that ts.
	found := false
	for _, c := range fake.Calls {
		if c.Name == "ssh" && len(c.Args) >= 2 &&
			strings.Contains(c.Args[1], "2026-01-01_120000") {
			found = true
			break
		}
	}
	require.True(t, found, "expected ssh call referencing explicit timestamp; got %+v", fake.Calls)
}

func TestRestoreUsesLatestWhenTimestampOmitted(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-01-01_120000", "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"k3s", "restore",
		"--config", cfgPath,
		"--yes",
	})

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "Restored 1 apps from 2026-05-11_120000")
}

func TestRestoreYesSkipsPrompt(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// No stdin provided — would block if prompt were shown.
	cmd.SetArgs([]string{"k3s", "restore", "--config", cfgPath, "--yes"})

	require.NoError(t, cmd.Execute())
	require.NotContains(t, out.String(), "[y/N]")
	require.Contains(t, out.String(), "Restored")
}

func TestRestorePromptRejectsOnN(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("n\n"))
	cmd.SetArgs([]string{"k3s", "restore", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	stdout := out.String()
	require.Contains(t, stdout, "[y/N]")
	require.Contains(t, stdout, "cancelled")
	require.NotContains(t, stdout, "Restored")

	// No ssh calls should have been made (Restore aborted before invoking manager).
	for _, c := range fake.Calls {
		require.NotEqual(t, "ssh", c.Name, "no ssh expected after cancellation, got %+v", c)
		require.NotEqual(t, "scp", c.Name, "no scp expected after cancellation, got %+v", c)
	}
}

func TestRestorePromptRejectsOnEmpty(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("\n"))
	cmd.SetArgs([]string{"k3s", "restore", "--config", cfgPath})

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "cancelled")
}

func TestRestorePromptAcceptsOnY(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{"k3s", "restore", "--config", cfgPath})

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "Restored")
}

func TestRestoreKubeContextMismatch(t *testing.T) {
	fake := &exec.FakeRunner{
		Responses: map[string]exec.FakeResponse{
			"kubectl config current-context": {Stdout: "wrong-ctx"},
		},
	}
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "restore", "--config", cfgPath, "--yes"})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "kubectl context mismatch")
}

func TestRestoreConfirmContextAllowsMismatch(t *testing.T) {
	fake := defaultRestoreFake()
	fake.Responses["kubectl config current-context"] = exec.FakeResponse{Stdout: "wrong-ctx"}
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "restore", "--config", cfgPath, "--yes", "--confirm-context"})

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "Restored")
}

func TestRestoreNoBackupsForTimestamp(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	// Create fixture with a timestamp dir but no tar files inside.
	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")
	localDir := filepath.Join(fixture, "backups")
	require.NoError(t, os.MkdirAll(filepath.Join(localDir, "2026-05-11_120000"), 0o755))
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	rewritten := strings.Replace(string(data),
		"localDir: testdata/backups",
		"localDir: "+localDir, 1)
	require.NoError(t, os.WriteFile(cfgPath, []byte(rewritten), 0o644))

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "restore", "--config", cfgPath, "--yes"})

	err = cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no apps")
}

func TestRestoreUnknownApp(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "restore", "ghost", "--config", cfgPath, "--yes"})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "ghost")
}

func TestRestoreExplicitApp(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "restore", "alpha", "--config", cfgPath, "--yes"})

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "Restored 1 apps from 2026-05-11_120000")
}

func TestRestoreLatestTimestampError(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	// Empty backups dir.
	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")
	localDir := filepath.Join(fixture, "backups")
	require.NoError(t, os.MkdirAll(localDir, 0o755))
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	rewritten := strings.Replace(string(data),
		"localDir: testdata/backups",
		"localDir: "+localDir, 1)
	require.NoError(t, os.WriteFile(cfgPath, []byte(rewritten), 0o644))

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "restore", "--config", cfgPath, "--yes"})

	err = cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no backups")
}

func TestRestoreConfigLoadError(t *testing.T) {
	fake := defaultRestoreFake()
	setupFakeRunner(t, fake)

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "restore", "--config", "/nonexistent.yaml", "--yes"})

	err := cmd.Execute()
	require.Error(t, err)
}

func TestRestoreManagerErrorPropagates(t *testing.T) {
	// Fail every ssh call.
	fake := &exec.FakeRunner{
		Responses: map[string]exec.FakeResponse{
			"kubectl config current-context": {Stdout: "test-ctx"},
		},
		Default: exec.FakeResponse{Err: fmt.Errorf("boom")},
	}
	setupFakeRunner(t, fake)

	cfgPath, _ := restoreFixture(t, "2026-05-11_120000")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "restore", "--config", cfgPath, "--yes"})

	err := cmd.Execute()
	require.Error(t, err)
}
