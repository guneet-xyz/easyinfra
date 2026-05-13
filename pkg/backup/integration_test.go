//go:build integration

package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/k8s"
)

// fsRunner is a fake exec runner that simulates ssh/scp by performing the
// equivalent operations on the local filesystem. It also returns canned
// responses for kubectl queries used by backup/restore.
//
// Layout used by the test:
//   - hostPath: directory acting as the PVC's host backing storage
//   - remoteTmp: directory acting as the SSH host's temp area
//   - localDir: directory acting as the operator's local backup dir
type fsRunner struct {
	hostPaths  map[string]string
	pvNames    map[string]string
	replicas   map[string]string
	deployByNS map[string]string
}

func (f *fsRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	full := name + " " + strings.Join(args, " ")

	switch name {
	case "kubectl":
		return f.handleKubectl(args, full)
	case "ssh":
		// args: [host, shellCmd]
		if len(args) < 2 {
			return "", "", nil
		}
		return f.handleShell(args[1])
	case "scp":
		return f.handleScp(args)
	}
	return "", "", fmt.Errorf("unexpected command: %s", full)
}

func (f *fsRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	_, _, err := f.Run(ctx, name, args...)
	return err
}

func (f *fsRunner) handleKubectl(args []string, full string) (string, string, error) {
	// kubectl get deployments -n <ns> -o jsonpath={.items[*].metadata.name}
	if len(args) >= 2 && args[0] == "get" && args[1] == "deployments" {
		ns := argAfter(args, "-n")
		if dep, ok := f.deployByNS[ns]; ok {
			return dep, "", nil
		}
		return "", "", nil
	}
	// kubectl get deployment <dep> -n <ns> -o jsonpath={.spec.replicas}
	if len(args) >= 2 && args[0] == "get" && args[1] == "deployment" {
		dep := args[2]
		ns := argAfter(args, "-n")
		key := ns + "/" + dep
		if r, ok := f.replicas[key]; ok {
			return r, "", nil
		}
		return "1", "", nil
	}
	// kubectl get pvc <name> -n <ns> -o jsonpath={.spec.volumeName}
	if len(args) >= 2 && args[0] == "get" && args[1] == "pvc" {
		pvc := args[2]
		if pv, ok := f.pvNames[pvc]; ok {
			return pv, "", nil
		}
		return "", "", fmt.Errorf("unknown pvc: %s", pvc)
	}
	// kubectl get pv <name> -o jsonpath={.spec.local.path}
	if len(args) >= 2 && args[0] == "get" && args[1] == "pv" {
		pvName := args[2]
		// reverse lookup pvc
		for pvc, p := range f.pvNames {
			if p == pvName {
				return f.hostPaths[pvc], "", nil
			}
		}
		return "", "", fmt.Errorf("unknown pv: %s", pvName)
	}
	// kubectl scale deployment ... --replicas=N => no-op
	if len(args) >= 1 && args[0] == "scale" {
		return "", "", nil
	}
	// kubectl wait --for=delete pod --all -n <ns> => no-op
	if len(args) >= 1 && args[0] == "wait" {
		return "", "", nil
	}
	return "", "", nil
}

// handleShell interprets remote shell commands sent via ssh.
// Recognized patterns:
//   mkdir -p <path>
//   rm -rf <path>
//   tar czf <archive> -C <dir> .
//   rm -rf '<path>'/* && tar xzf <archive> -C '<path>'
func (f *fsRunner) handleShell(cmd string) (string, string, error) {
	cmd = strings.TrimSpace(cmd)

	// Compound restore: rm -rf '<path>'/* && tar xzf <archive> -C '<path>'
	if strings.Contains(cmd, "&&") && strings.Contains(cmd, "tar xzf") {
		parts := strings.SplitN(cmd, "&&", 2)
		for _, p := range parts {
			if _, _, err := f.handleShell(strings.TrimSpace(p)); err != nil {
				return "", "", err
			}
		}
		return "", "", nil
	}

	switch {
	case strings.HasPrefix(cmd, "mkdir -p "):
		path := strings.TrimSpace(strings.TrimPrefix(cmd, "mkdir -p "))
		path = strings.Trim(path, "'\"")
		return "", "", os.MkdirAll(path, 0755)

	case strings.HasPrefix(cmd, "rm -rf "):
		// Could be rm -rf <dir> or rm -rf '<dir>'/*
		rest := strings.TrimSpace(strings.TrimPrefix(cmd, "rm -rf "))
		if strings.HasSuffix(rest, "/*") {
			dir := strings.TrimSuffix(rest, "/*")
			dir = strings.Trim(dir, "'\"")
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					return "", "", nil
				}
				return "", "", err
			}
			for _, e := range entries {
				if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
					return "", "", err
				}
			}
			return "", "", nil
		}
		path := strings.Trim(rest, "'\"")
		return "", "", os.RemoveAll(path)

	case strings.HasPrefix(cmd, "tar czf "):
		// tar czf <archive> -C <dir> .
		fields := splitFields(cmd)
		// ["tar","czf","<archive>","-C","<dir>","."]
		if len(fields) < 6 {
			return "", "", fmt.Errorf("malformed tar czf: %q", cmd)
		}
		archive := fields[2]
		dir := fields[4]
		return "", "", runReal("tar", "czf", archive, "-C", dir, ".")

	case strings.HasPrefix(cmd, "tar xzf "):
		// tar xzf <archive> -C <dir>
		fields := splitFields(cmd)
		if len(fields) < 5 {
			return "", "", fmt.Errorf("malformed tar xzf: %q", cmd)
		}
		archive := fields[2]
		dir := fields[4]
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", "", err
		}
		return "", "", runReal("tar", "xzf", archive, "-C", dir)
	}

	return "", "", fmt.Errorf("unhandled remote shell command: %q", cmd)
}

// handleScp simulates scp transfers between "local" and "remote" temp dirs.
// All paths are real local paths in the test (the "remote" is just another dir).
//
// Patterns:
//   scp -r host:remoteSrc/* localDir/   (backup pull)
//   scp localTar host:remoteDest        (restore push)
func (f *fsRunner) handleScp(args []string) (string, string, error) {
	// Strip leading -r flag if present
	recursive := false
	if len(args) > 0 && args[0] == "-r" {
		recursive = true
		args = args[1:]
	}
	if len(args) < 2 {
		return "", "", fmt.Errorf("scp needs src and dst")
	}
	src := args[0]
	dst := args[1]

	src = stripHost(src)
	dst = stripHost(dst)

	if strings.HasSuffix(src, "/*") {
		// Copy contents of src dir into dst dir.
		srcDir := strings.TrimSuffix(src, "/*")
		dstDir := strings.TrimSuffix(dst, "/")
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return "", "", err
		}
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			return "", "", err
		}
		for _, e := range entries {
			s := filepath.Join(srcDir, e.Name())
			d := filepath.Join(dstDir, e.Name())
			if err := copyAny(s, d); err != nil {
				return "", "", err
			}
		}
		return "", "", nil
	}

	// Plain file (or recursive dir) copy.
	if recursive {
		return "", "", copyAny(src, dst)
	}
	return "", "", copyFile(src, dst)
}

// stripHost removes a "user@host:" or "host:" prefix from an scp path.
func stripHost(p string) string {
	if i := strings.Index(p, ":"); i != -1 {
		// Heuristic: if it looks like host:path (not a windows drive), strip.
		// All paths in this test are unix-style, so this is safe.
		return p[i+1:]
	}
	return p
}

func copyAny(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			if err := copyFile(s, d); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func runReal(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %v: %s", name, args, err, string(out))
	}
	return nil
}

func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// splitFields splits a shell-ish command on whitespace, stripping single/double
// quotes from each field. Sufficient for the constrained commands this test
// produces.
func splitFields(s string) []string {
	raw := strings.Fields(s)
	out := make([]string, len(raw))
	for i, f := range raw {
		out[i] = strings.Trim(f, "'\"")
	}
	return out
}

// TestRoundTrip exercises the full backup → mutate → restore flow end-to-end
// using a filesystem-backed fake runner that simulates ssh/scp/tar against
// real temp directories.
func TestRoundTrip(t *testing.T) {
	// Skip if tar is not available (Windows).
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar binary not available")
	}

	// "Remote" PVC host path with original data.
	hostPath := t.TempDir()
	originalContent := []byte("hello round trip\n")
	dataFile := filepath.Join(hostPath, "data.txt")
	if err := os.WriteFile(dataFile, originalContent, 0644); err != nil {
		t.Fatalf("seed data: %v", err)
	}

	// "Remote" tmp area used by ssh/scp simulator.
	remoteTmp := t.TempDir()
	// Operator-side local backup directory.
	localDir := t.TempDir()

	runner := &fsRunner{
		hostPaths: map[string]string{
			"alpha-data": hostPath,
		},
		pvNames: map[string]string{
			"alpha-data": "pv-alpha",
		},
		replicas: map[string]string{
			"alpha/alpha-deploy": "2",
		},
		deployByNS: map[string]string{
			"alpha": "alpha-deploy",
		},
	}

	mgr := &Manager{
		Runner: runner,
		K8s:    &k8s.Client{Runner: runner},
		Cfg: config.BackupConfig{
			RemoteHost: "test-host",
			RemoteTmp:  remoteTmp,
			LocalDir:   localDir,
		},
	}

	apps := []config.AppConfig{
		{Name: "alpha", Namespace: "alpha", PVCs: []string{"alpha-data"}},
	}

	ctx := context.Background()

	// Step 1: backup.
	ts, err := mgr.Run(ctx, apps)
	if err != nil {
		t.Fatalf("backup Run failed: %v", err)
	}
	if ts == "" {
		t.Fatal("expected non-empty timestamp")
	}

	// The local backup dir should contain alpha-data.tar.gz.
	tarPath := filepath.Join(localDir, ts, "alpha-data.tar.gz")
	if _, err := os.Stat(tarPath); err != nil {
		t.Fatalf("expected backup tar at %s: %v", tarPath, err)
	}

	// Step 2: mutate the live data.
	if err := os.WriteFile(dataFile, []byte("CORRUPTED\n"), 0644); err != nil {
		t.Fatalf("mutate data: %v", err)
	}

	// Step 3: restore (latest).
	if err := mgr.Restore(ctx, apps, ""); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// Step 4: verify the original content is back.
	got, err := os.ReadFile(dataFile)
	if err != nil {
		t.Fatalf("read restored data: %v", err)
	}
	if string(got) != string(originalContent) {
		t.Fatalf("data mismatch after restore:\n got: %q\nwant: %q", string(got), string(originalContent))
	}
}
