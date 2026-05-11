package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/guneet-xyz/easyinfra/pkg/repo"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func withFakeRunner(t *testing.T, fake *exec.FakeRunner) string {
	t.Helper()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	orig := newRepoManager
	newRepoManager = func(_ *cobra.Command, _ *rootFlags) (*repo.Manager, error) {
		return &repo.Manager{Runner: fake, RepoDir: repoDir}, nil
	}
	t.Cleanup(func() { newRepoManager = orig })
	return repoDir
}

func TestInitClonesWithBranch(t *testing.T) {
	fake := &exec.FakeRunner{}
	repoDir := withFakeRunner(t, fake)

	var out, errb bytes.Buffer
	cmd := newRootCmd("dev", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"init", "https://example.com/r.git", "--branch", "main"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Len(t, fake.Calls, 1)
	require.Equal(t, "git", fake.Calls[0].Name)
	require.Equal(t, []string{"clone", "--branch", "main", "https://example.com/r.git", repoDir}, fake.Calls[0].Args)
	require.Contains(t, out.String(), "Cloned https://example.com/r.git")
	require.Contains(t, out.String(), repoDir)
}

func TestInitClonesNoBranch(t *testing.T) {
	fake := &exec.FakeRunner{}
	repoDir := withFakeRunner(t, fake)

	var out bytes.Buffer
	cmd := newRootCmd("dev", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"init", "https://example.com/r.git"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, []string{"clone", "https://example.com/r.git", repoDir}, fake.Calls[0].Args)
}

func TestInitRequiresURL(t *testing.T) {
	fake := &exec.FakeRunner{}
	withFakeRunner(t, fake)

	var out bytes.Buffer
	cmd := newRootCmd("dev", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"init"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestInitExistingNoForce(t *testing.T) {
	fake := &exec.FakeRunner{}
	repoDir := withFakeRunner(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))

	var out bytes.Buffer
	cmd := newRootCmd("dev", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"init", "https://example.com/r.git"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--force")
}

func TestInitForceOverwrites(t *testing.T) {
	fake := &exec.FakeRunner{}
	repoDir := withFakeRunner(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))

	var out bytes.Buffer
	cmd := newRootCmd("dev", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"init", "https://example.com/r.git", "--force"})
	err := cmd.Execute()
	require.NoError(t, err)
	require.Len(t, fake.Calls, 1)
}

func TestDefaultNewRepoManager(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newRootCmd("dev", "x", "x")
	flags := &rootFlags{}
	mgr, err := newRepoManager(cmd, flags)
	require.NoError(t, err)
	require.NotNil(t, mgr)
	require.NotEmpty(t, mgr.RepoDir)
	require.NotNil(t, mgr.Runner)
}
