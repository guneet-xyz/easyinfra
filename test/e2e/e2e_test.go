//go:build e2e

package e2e

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binaryPath is the absolute path to the built easyinfra binary.
// Set by TestMain.
var binaryPath string

// repoRoot is the absolute path to the easyinfra repo root.
// Set by TestMain.
var repoRoot string

// testdataBin is the absolute path to testdata/bin (fake helm/kubectl).
// Set by TestMain.
var testdataBin string

func TestMain(m *testing.M) {
	// Tests run from test/e2e; resolve repo root from there.
	wd, err := os.Getwd()
	if err != nil {
		panic("getwd: " + err.Error())
	}
	repoRoot = filepath.Clean(filepath.Join(wd, "..", ".."))
	testdataBin = filepath.Join(repoRoot, "testdata", "bin")

	binDir := filepath.Join(repoRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		panic("mkdir bin: " + err.Error())
	}
	binaryPath = filepath.Join(binDir, "easyinfra")

	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/easyinfra")
	build.Dir = repoRoot
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("build easyinfra: " + err.Error())
	}

	os.Exit(m.Run())
}

// runEasyinfra executes the built easyinfra binary with the given args.
// PATH is set so that testdata/bin (fake helm/kubectl) is found first,
// followed by the bin/ dir, then the inherited PATH.
// XDG_CONFIG_HOME may be overridden via env (key=value pairs).
func runEasyinfra(t *testing.T, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = repoRoot

	path := testdataBin + string(os.PathListSeparator) +
		filepath.Join(repoRoot, "bin") + string(os.PathListSeparator) +
		os.Getenv("PATH")

	baseEnv := os.Environ()
	baseEnv = append(baseEnv, "PATH="+path)
	baseEnv = append(baseEnv, env...)
	cmd.Env = baseEnv

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("running easyinfra: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// makeTempXDG returns env vars for a test-isolated XDG_CONFIG_HOME and HOME.
func makeTempXDG(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	xdg := filepath.Join(dir, "config")
	if err := os.MkdirAll(xdg, 0o755); err != nil {
		t.Fatalf("mkdir xdg: %v", err)
	}
	return []string{
		"XDG_CONFIG_HOME=" + xdg,
		"HOME=" + dir,
	}
}

// makeLocalGitRepo creates a tiny local git repo with a single commit,
// returns a file:// URL pointing at it.
func makeLocalGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(name string, args ...string) {
		t.Helper()
		c := exec.Command(name, args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_TERMINAL_PROMPT=0",
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}

	run("git", "init", "-b", "main", ".")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "test")

	if err := os.WriteFile(filepath.Join(dir, "infra.yaml"), []byte("kubeContext: test-ctx\napps: []\n"), 0o644); err != nil {
		t.Fatalf("write infra.yaml: %v", err)
	}
	run("git", "add", ".")
	run("git", "commit", "-m", "initial")

	return "file://" + dir
}

func TestVersion(t *testing.T) {
	stdout, _, code := runEasyinfra(t, nil, "version")
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q", code, stdout)
	}
	if !strings.Contains(stdout, "easyinfra") {
		t.Fatalf("expected stdout to contain %q, got %q", "easyinfra", stdout)
	}
}

func TestInitFresh(t *testing.T) {
	env := makeTempXDG(t)
	url := makeLocalGitRepo(t)

	stdout, stderr, code := runEasyinfra(t, env, "init", url)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Cloned") {
		t.Fatalf("expected stdout to contain 'Cloned', got %q", stdout)
	}

	// Verify infra.yaml exists in the cloned repo.
	// XDG_CONFIG_HOME is in env[0] in form KEY=VAL.
	xdg := strings.TrimPrefix(env[0], "XDG_CONFIG_HOME=")
	infraPath := filepath.Join(xdg, "easyinfra", "repo", "infra.yaml")
	if _, err := os.Stat(infraPath); err != nil {
		t.Fatalf("expected cloned infra.yaml at %s: %v", infraPath, err)
	}
}

func TestInitExistingRefuses(t *testing.T) {
	env := makeTempXDG(t)
	url := makeLocalGitRepo(t)

	if _, _, code := runEasyinfra(t, env, "init", url); code != 0 {
		t.Fatalf("first init failed: exit=%d", code)
	}
	stdout, stderr, code := runEasyinfra(t, env, "init", url)
	if code == 0 {
		t.Fatalf("expected second init to fail without --force")
	}
	combined := stdout + stderr
	if combined != "" && !strings.Contains(combined, "already exists") && !strings.Contains(combined, "force") {
		t.Fatalf("expected error to mention --force, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestInitForceReplaces(t *testing.T) {
	env := makeTempXDG(t)
	url := makeLocalGitRepo(t)

	if _, _, code := runEasyinfra(t, env, "init", url); code != 0 {
		t.Fatalf("first init failed: exit=%d", code)
	}
	stdout, stderr, code := runEasyinfra(t, env, "init", "--force", url)
	if code != 0 {
		t.Fatalf("init --force failed: exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "Cloned") {
		t.Fatalf("expected stdout to contain 'Cloned', got %q", stdout)
	}
}

func TestUpdate(t *testing.T) {
	env := makeTempXDG(t)
	url := makeLocalGitRepo(t)

	if _, _, code := runEasyinfra(t, env, "init", url); code != 0 {
		t.Fatalf("init failed: exit=%d", code)
	}
	stdout, stderr, code := runEasyinfra(t, env, "update")
	if code != 0 {
		t.Fatalf("update failed: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Updated") {
		t.Fatalf("expected stdout to contain 'Updated', got %q", stdout)
	}
}

func TestK3sValidate(t *testing.T) {
	stdout, stderr, code := runEasyinfra(t, nil, "k3s", "validate", "--config", "testdata/infra/infra.yaml")
	if code != 0 {
		t.Fatalf("validate failed: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "validated") {
		t.Fatalf("expected stdout to contain 'validated', got %q", stdout)
	}
}

func TestK3sInstallDryRun(t *testing.T) {
	stdout, stderr, code := runEasyinfra(t, nil,
		"--dry-run", "--confirm-context",
		"k3s", "install", "alpha",
		"--config", "testdata/infra/infra.yaml",
	)
	if code != 0 {
		t.Fatalf("install dry-run failed: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "helm") || !strings.Contains(stdout, "install alpha") {
		t.Fatalf("expected helm install alpha in stdout, got %q", stdout)
	}
}

func TestK3sUpgradeDryRun(t *testing.T) {
	stdout, stderr, code := runEasyinfra(t, nil,
		"--dry-run", "--confirm-context",
		"k3s", "upgrade", "alpha",
		"--config", "testdata/infra/infra.yaml",
	)
	if code != 0 {
		t.Fatalf("upgrade dry-run failed: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "helm") || !strings.Contains(stdout, "upgrade alpha") {
		t.Fatalf("expected helm upgrade alpha in stdout, got %q", stdout)
	}
}

func TestK3sUninstallAllReverse(t *testing.T) {
	stdout, stderr, code := runEasyinfra(t, nil,
		"--dry-run", "--confirm-context",
		"k3s", "uninstall", "--all", "--yes",
		"--config", "testdata/infra/infra.yaml",
	)
	if code != 0 {
		t.Fatalf("uninstall dry-run failed: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	betaIdx := strings.Index(stdout, "uninstall beta")
	alphaIdx := strings.Index(stdout, "uninstall alpha")
	if betaIdx < 0 || alphaIdx < 0 {
		t.Fatalf("expected uninstall beta and uninstall alpha in stdout, got %q", stdout)
	}
	if betaIdx > alphaIdx {
		t.Fatalf("expected beta uninstalled before alpha (reverse order), got:\n%s", stdout)
	}
}

func TestK3sBackupDryRun(t *testing.T) {
	stdout, _, _ := runEasyinfra(t, nil,
		"--dry-run", "--confirm-context",
		"k3s", "backup",
		"--config", "testdata/infra/infra.yaml",
	)
	// Backup dry-run may exit non-zero because fake kubectl returns empty
	// volume names; we only verify orchestration commands were emitted.
	for _, want := range []string{"ssh", "kubectl", "scp"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected stdout to contain %q, got:\n%s", want, stdout)
		}
	}
}

func TestK3sRestoreDryRun(t *testing.T) {
	// Restore requires at least one tarball matching the timestamp; create
	// one in testdata/backups (the configured localDir) and clean it up.
	timestamp := "2024-01-01_120000"
	tsDir := filepath.Join(repoRoot, "testdata", "backups", timestamp)
	if err := os.MkdirAll(tsDir, 0o755); err != nil {
		t.Fatalf("mkdir backups: %v", err)
	}
	tarball := filepath.Join(tsDir, "alpha-data.tar.gz")
	if err := os.WriteFile(tarball, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write tarball: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tsDir)
		// Also remove parent if empty.
		_ = os.Remove(filepath.Join(repoRoot, "testdata", "backups"))
	})

	stdout, _, _ := runEasyinfra(t, nil,
		"--dry-run", "--confirm-context",
		"k3s", "restore", "--yes", "--timestamp", timestamp,
		"--config", "testdata/infra/infra.yaml",
	)
	for _, want := range []string{"ssh", "kubectl", "scp"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected stdout to contain %q, got:\n%s", want, stdout)
		}
	}
}

func TestKubeContextMismatch(t *testing.T) {
	// Write a config with a wrong kubeContext.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "infra.yaml")
	cfg := "kubeContext: wrong-ctx\napps:\n  - name: alpha\n    chart: charts/alpha\n    namespace: alpha\n    order: 1\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	stdout, stderr, code := runEasyinfra(t, nil,
		"k3s", "install", "alpha",
		"--config", cfgPath,
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit on kube context mismatch; stdout=%q stderr=%q", stdout, stderr)
	}
	combined := stdout + stderr
	if combined != "" && !strings.Contains(combined, "--confirm-context") && !strings.Contains(combined, "kubectl") {
		t.Fatalf("expected error to mention --confirm-context, got:\nstdout=%q\nstderr=%q", stdout, stderr)
	}
}

func TestUnknownAppError(t *testing.T) {
	stdout, stderr, code := runEasyinfra(t, nil,
		"k3s", "install", "nonexistent",
		"--config", "testdata/infra/infra.yaml",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown app; stdout=%q stderr=%q", stdout, stderr)
	}
	combined := stdout + stderr
	if combined != "" && !strings.Contains(combined, "alpha") && !strings.Contains(combined, "kubectl") {
		t.Fatalf("expected output to list known apps (alpha, beta), got stdout=%q stderr=%q", stdout, stderr)
	}
}
