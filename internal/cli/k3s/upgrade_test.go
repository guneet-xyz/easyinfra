package k3s

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpgradeAllApps(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "upgrade", "--all", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	upgrades := helmCalls(fake, "upgrade")
	require.Len(t, upgrades, 2)
	require.Equal(t, "alpha", helmRelease(upgrades[0], "upgrade"))
	require.Equal(t, "beta", helmRelease(upgrades[1], "upgrade"))

	for _, c := range upgrades {
		for _, a := range c.Args {
			require.NotEqual(t, "--create-namespace", a, "upgrade must not pass --create-namespace")
		}
	}

	require.Contains(t, out.String(), "Upgraded alpha")
	require.Contains(t, out.String(), "Upgraded beta")
}

func TestUpgradeSingleApp(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "upgrade", "alpha", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	upgrades := helmCalls(fake, "upgrade")
	require.Len(t, upgrades, 1)
	require.Equal(t, "alpha", helmRelease(upgrades[0], "upgrade"))
}

func TestUpgradeUnknownApp(t *testing.T) {
	fake := okContextRunner()
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "upgrade", "ghost", "--config", cfgPath})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown app")
}
