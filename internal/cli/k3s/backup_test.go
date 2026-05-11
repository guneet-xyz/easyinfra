package k3s

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// backupFakeResponses returns the canned kubectl/ssh/scp responses needed by
// the backup orchestration (replica lookups, PVC/PV resolution, current context).
func backupFakeResponses() map[string]exec.FakeResponse {
	return map[string]exec.FakeResponse{
		"kubectl config current-context": {Stdout: "test-ctx"},
		"kubectl get deployments -n alpha -o jsonpath={.items[*].metadata.name}": {Stdout: "alpha-deploy"},
		"kubectl get deployments -n beta -o jsonpath={.items[*].metadata.name}":  {Stdout: ""},
		"kubectl get deployment alpha-deploy -n alpha -o jsonpath={.spec.replicas}": {Stdout: "1"},
		"kubectl get pvc alpha-data -n alpha -o jsonpath={.spec.volumeName}":        {Stdout: "pv-alpha"},
		"kubectl get pv pv-alpha -o jsonpath={.spec.local.path}":                    {Stdout: "/var/lib/alpha"},
	}
}

func TestBackupAllAppsSucceeds(t *testing.T) {
	fake := &exec.FakeRunner{
		Default:   exec.FakeResponse{Stdout: "ok"},
		Responses: backupFakeResponses(),
	}
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out, errb bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"k3s", "backup", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	stdout := out.String()
	require.Contains(t, stdout, "skip: beta — no pvcs")
	require.Contains(t, stdout, "Backup complete:")

	// alpha must have been processed: scale down, tar, scale up.
	require.True(t, hasCallSubstr(fake.Calls, "scale deployment alpha-deploy -n alpha --replicas=0"))
	require.True(t, hasCallSubstr(fake.Calls, "tar czf"))
	require.True(t, hasCallSubstr(fake.Calls, "scale deployment alpha-deploy -n alpha --replicas=1"))
}

func TestBackupFilteredByAppName(t *testing.T) {
	fake := &exec.FakeRunner{
		Default:   exec.FakeResponse{Stdout: "ok"},
		Responses: backupFakeResponses(),
	}
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "backup", "--config", cfgPath, "alpha"})

	require.NoError(t, cmd.Execute())

	stdout := out.String()
	require.Contains(t, stdout, "Backup complete:")
	require.NotContains(t, stdout, "beta")
	require.True(t, hasCallSubstr(fake.Calls, "scale deployment alpha-deploy -n alpha --replicas=0"))
	// beta namespace should not be touched
	for _, c := range fake.Calls {
		require.NotContains(t, callStr(c), "-n beta")
	}
}

func TestBackupAppWithoutPVCsIsSkipped(t *testing.T) {
	fake := &exec.FakeRunner{
		Default:   exec.FakeResponse{Stdout: "ok"},
		Responses: backupFakeResponses(),
	}
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "backup", "--config", cfgPath, "beta"})

	require.NoError(t, cmd.Execute())

	stdout := out.String()
	require.Contains(t, stdout, "skip: beta — no pvcs")
	// Nothing should have been backed up: no tar, no scale.
	for _, c := range fake.Calls {
		s := callStr(c)
		require.NotContains(t, s, "tar czf")
		require.NotContains(t, s, "kubectl scale")
	}
}

func TestBackupPartialFailureExitsNonZero(t *testing.T) {
	// Fail the tar step. The Manager records this as
	//   "backup alpha: tar alpha-data: <err>"
	// which is then joined into the returned error.
	resp := backupFakeResponses()
	fake := &exec.FakeRunner{
		Default:   exec.FakeResponse{Stdout: "ok"},
		Responses: resp,
	}
	// Wrap the fake with a runner that fails ssh tar invocations.
	failingRunner := &substringFailingRunner{
		inner:    fake,
		failSubs: []string{"tar czf"},
		failErr:  errors.New("ssh tar failed"),
	}
	orig := newRunner
	newRunner = func(_ *cobra.Command, _ *RootFlags) exec.Runner { return failingRunner }
	t.Cleanup(func() { newRunner = orig })

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out, errb bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"k3s", "backup", "--config", cfgPath})

	err := cmd.Execute()
	require.Error(t, err)

	stdout := out.String()
	require.Contains(t, stdout, "Failed:")
	require.Contains(t, stdout, "alpha")
	require.NotContains(t, stdout, "Backup complete:")
}

func TestBackupUnknownAppErrors(t *testing.T) {
	fake := &exec.FakeRunner{
		Default:   exec.FakeResponse{Stdout: "ok"},
		Responses: backupFakeResponses(),
	}
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out, errb bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"k3s", "backup", "--config", cfgPath, "ghost"})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "ghost")
}

func TestBackupKubeContextMismatch(t *testing.T) {
	resp := backupFakeResponses()
	resp["kubectl config current-context"] = exec.FakeResponse{Stdout: "wrong-ctx"}
	fake := &exec.FakeRunner{
		Default:   exec.FakeResponse{Stdout: "ok"},
		Responses: resp,
	}
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out, errb bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"k3s", "backup", "--config", cfgPath})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "context mismatch")
}

// --- helpers ---

func callStr(c exec.FakeCall) string {
	return strings.Join(append([]string{c.Name}, c.Args...), " ")
}

func hasCallSubstr(calls []exec.FakeCall, sub string) bool {
	for _, c := range calls {
		if strings.Contains(callStr(c), sub) {
			return true
		}
	}
	return false
}

// substringFailingRunner wraps another runner and returns failErr whenever the
// concatenated "name args..." contains any of failSubs.
type substringFailingRunner struct {
	inner    exec.Runner
	failSubs []string
	failErr  error
}

func (r *substringFailingRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	full := strings.Join(append([]string{name}, args...), " ")
	for _, s := range r.failSubs {
		if strings.Contains(full, s) {
			// Record the call so tests can still inspect it via the inner fake.
			if fr, ok := r.inner.(*exec.FakeRunner); ok {
				fr.Calls = append(fr.Calls, exec.FakeCall{Name: name, Args: args})
			}
			return "", "", r.failErr
		}
	}
	return r.inner.Run(ctx, name, args...)
}

func (r *substringFailingRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	_, _, err := r.Run(ctx, name, args...)
	return err
}
