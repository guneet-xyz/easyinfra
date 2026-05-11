package k3s

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/stretchr/testify/require"
)

func okContextRunner() *exec.FakeRunner {
	return &exec.FakeRunner{
		Responses: map[string]exec.FakeResponse{
			"kubectl config current-context": {Stdout: "test-ctx"},
		},
		Default: exec.FakeResponse{Stdout: "ok"},
	}
}

func helmCalls(fake *exec.FakeRunner, sub string) []exec.FakeCall {
	var out []exec.FakeCall
	for _, c := range fake.Calls {
		if c.Name != "helm" {
			continue
		}
		for _, a := range c.Args {
			if a == sub {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func helmRelease(call exec.FakeCall, verb string) string {
	for i, a := range call.Args {
		if a == verb && i+1 < len(call.Args) {
			return call.Args[i+1]
		}
	}
	return ""
}

func TestInstallAllApps(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "install", "--all", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	stdout := out.String()
	require.Contains(t, stdout, "Installed alpha")
	require.Contains(t, stdout, "Installed beta")

	installs := helmCalls(fake, "install")
	require.Len(t, installs, 2)

	require.Equal(t, "alpha", helmRelease(installs[0], "install"))
	require.Equal(t, "beta", helmRelease(installs[1], "install"))

	a := strings.Join(installs[0].Args, " ")
	require.Contains(t, a, "-n alpha")
	require.Contains(t, a, "--atomic")
	require.Contains(t, a, "--wait")
	require.Contains(t, a, "--create-namespace")
	require.Contains(t, a, "values-shared.yaml")
	require.Contains(t, a, "charts/alpha/values.yaml")
	require.Contains(t, a, "--post-renderer echo")
	require.Contains(t, a, "--post-renderer-args noop")
}

func TestInstallSingleApp(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "install", "beta", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	installs := helmCalls(fake, "install")
	require.Len(t, installs, 1)
	require.Equal(t, "beta", helmRelease(installs[0], "install"))
	require.Contains(t, out.String(), "Installed beta")
}

func TestInstallUnknownApp(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "install", "ghost", "--config", cfgPath})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown app")
	require.Contains(t, err.Error(), "alpha")
	require.Contains(t, err.Error(), "beta")
}

func TestInstallRequiresAppOrAll(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "install", "--config", cfgPath})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--all")
}

func TestInstallContextMismatchFails(t *testing.T) {
	fake := &exec.FakeRunner{
		Responses: map[string]exec.FakeResponse{
			"kubectl config current-context": {Stdout: "wrong-ctx"},
		},
		Default: exec.FakeResponse{Stdout: "ok"},
	}
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "install", "--all", "--config", cfgPath})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "context mismatch")

	require.Empty(t, helmCalls(fake, "install"))
}
