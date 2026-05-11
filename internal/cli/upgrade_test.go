package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/selfupdate"
	"github.com/stretchr/testify/require"
)

// mockUpdater implements updaterInterface for testing
type mockUpdater struct {
	checkResult *selfupdate.CheckResult
	checkErr    error
	updateErr   error
	updateVer   string
}

func (m *mockUpdater) Check(_ context.Context) (*selfupdate.CheckResult, error) {
	return m.checkResult, m.checkErr
}

func (m *mockUpdater) Update(_ context.Context) (string, error) {
	return m.updateVer, m.updateErr
}

func withMockUpdater(t *testing.T, mock *mockUpdater) {
	t.Helper()
	orig := newUpdater
	newUpdater = func(_ string) updaterInterface {
		return mock
	}
	t.Cleanup(func() { newUpdater = orig })
}

func TestUpgradeAlreadyLatest(t *testing.T) {
	mock := &mockUpdater{
		checkResult: &selfupdate.CheckResult{
			LatestVersion: "v1.0.0",
			HasUpdate:     false,
		},
	}
	withMockUpdater(t, mock)

	var out bytes.Buffer
	cmd := newRootCmd("v1.0.0", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, out.String(), "Already on latest (v1.0.0)")
}

func TestUpgradeCheckFlag(t *testing.T) {
	mock := &mockUpdater{
		checkResult: &selfupdate.CheckResult{
			LatestVersion: "v1.1.0",
			HasUpdate:     true,
		},
	}
	withMockUpdater(t, mock)

	var out bytes.Buffer
	cmd := newRootCmd("v1.0.0", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "--check"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, out.String(), "Update available: v1.0.0 → v1.1.0")
}

func TestUpgradeCheckFlagNoUpdate(t *testing.T) {
	mock := &mockUpdater{
		checkResult: &selfupdate.CheckResult{
			LatestVersion: "v1.0.0",
			HasUpdate:     false,
		},
	}
	withMockUpdater(t, mock)

	var out bytes.Buffer
	cmd := newRootCmd("v1.0.0", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "--check"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, out.String(), "Already on latest (v1.0.0)")
}

func TestUpgradeWithYesFlag(t *testing.T) {
	mock := &mockUpdater{
		checkResult: &selfupdate.CheckResult{
			LatestVersion: "v1.1.0",
			HasUpdate:     true,
		},
		updateVer: "v1.1.0",
	}
	withMockUpdater(t, mock)

	var out bytes.Buffer
	cmd := newRootCmd("v1.0.0", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "--yes"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, out.String(), "Upgraded to v1.1.0")
}

func TestUpgradeCheckError(t *testing.T) {
	mock := &mockUpdater{
		checkErr: errors.New("network error"),
	}
	withMockUpdater(t, mock)

	var out bytes.Buffer
	cmd := newRootCmd("v1.0.0", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "--check"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "network error")
}

func TestUpgradeUpdateError(t *testing.T) {
	mock := &mockUpdater{
		checkResult: &selfupdate.CheckResult{
			LatestVersion: "v1.1.0",
			HasUpdate:     true,
		},
		updateErr: errors.New("download failed"),
	}
	withMockUpdater(t, mock)

	var out bytes.Buffer
	cmd := newRootCmd("v1.0.0", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "--yes"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "download failed")
}

// TestUpgradePromptYes tests interactive prompt with "y" response
func TestUpgradePromptYes(t *testing.T) {
	mock := &mockUpdater{
		checkResult: &selfupdate.CheckResult{
			LatestVersion: "v1.1.0",
			HasUpdate:     true,
		},
		updateVer: "v1.1.0",
	}
	withMockUpdater(t, mock)

	var out bytes.Buffer
	var in bytes.Buffer
	in.WriteString("y\n")

	cmd := newRootCmd("v1.0.0", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(&in)
	cmd.SetArgs([]string{"upgrade"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, out.String(), "Update easyinfra v1.0.0 → v1.1.0?")
	require.Contains(t, out.String(), "Upgraded to v1.1.0")
}

// TestUpgradePromptNo tests interactive prompt with "n" response
func TestUpgradePromptNo(t *testing.T) {
	mock := &mockUpdater{
		checkResult: &selfupdate.CheckResult{
			LatestVersion: "v1.1.0",
			HasUpdate:     true,
		},
	}
	withMockUpdater(t, mock)

	var out bytes.Buffer
	var in bytes.Buffer
	in.WriteString("n\n")

	cmd := newRootCmd("v1.0.0", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(&in)
	cmd.SetArgs([]string{"upgrade"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, out.String(), "Update easyinfra v1.0.0 → v1.1.0?")
	require.Contains(t, out.String(), "Upgrade cancelled")
}

// TestUpgradePromptEmpty tests interactive prompt with empty response (default no)
func TestUpgradePromptEmpty(t *testing.T) {
	mock := &mockUpdater{
		checkResult: &selfupdate.CheckResult{
			LatestVersion: "v1.1.0",
			HasUpdate:     true,
		},
	}
	withMockUpdater(t, mock)

	var out bytes.Buffer
	var in bytes.Buffer
	in.WriteString("\n")

	cmd := newRootCmd("v1.0.0", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(&in)
	cmd.SetArgs([]string{"upgrade"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, out.String(), "Upgrade cancelled")
}
