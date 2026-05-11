package k3s

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUninstallSingleApp(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "uninstall", "alpha", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	uninstalls := helmCalls(fake, "uninstall")
	require.Len(t, uninstalls, 1)
	require.Equal(t, "alpha", helmRelease(uninstalls[0], "uninstall"))
	require.Contains(t, out.String(), "Uninstalled alpha")
}

func TestUninstallAllReverseOrderWithYes(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "uninstall", "--all", "--yes", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	uninstalls := helmCalls(fake, "uninstall")
	require.Len(t, uninstalls, 2)
	require.Equal(t, "beta", helmRelease(uninstalls[0], "uninstall"))
	require.Equal(t, "alpha", helmRelease(uninstalls[1], "uninstall"))
}

func TestUninstallAllPromptDeclines(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("n\n"))
	cmd.SetArgs([]string{"k3s", "uninstall", "--all", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	require.Empty(t, helmCalls(fake, "uninstall"))
	require.Contains(t, out.String(), "cancelled")
}

func TestUninstallAllPromptAccepts(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{"k3s", "uninstall", "--all", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	uninstalls := helmCalls(fake, "uninstall")
	require.Len(t, uninstalls, 2)
}

func TestUninstallUnknownApp(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "uninstall", "ghost", "--config", cfgPath})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown app")
}
