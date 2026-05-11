package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/stretchr/testify/require"
)

func newManager(t *testing.T, runner exec.Runner) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	return &Manager{Runner: runner, RepoDir: repoDir}, repoDir
}

func TestCloneArgs(t *testing.T) {
	fake := &exec.FakeRunner{}
	m, repoDir := newManager(t, fake)
	err := m.Clone(context.Background(), "https://example.com/repo.git", "main", false)
	require.NoError(t, err)
	require.Len(t, fake.Calls, 1)
	require.Equal(t, "git", fake.Calls[0].Name)
	require.Equal(t, []string{"clone", "--branch", "main", "https://example.com/repo.git", repoDir}, fake.Calls[0].Args)
}

func TestCloneNoBranch(t *testing.T) {
	fake := &exec.FakeRunner{}
	m, repoDir := newManager(t, fake)
	err := m.Clone(context.Background(), "https://example.com/repo.git", "", false)
	require.NoError(t, err)
	require.Equal(t, []string{"clone", "https://example.com/repo.git", repoDir}, fake.Calls[0].Args)
}

func TestCloneExistingRefuses(t *testing.T) {
	fake := &exec.FakeRunner{}
	m, repoDir := newManager(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))
	err := m.Clone(context.Background(), "https://example.com/repo.git", "", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--force")
}

func TestCloneForceOverwrites(t *testing.T) {
	fake := &exec.FakeRunner{}
	m, repoDir := newManager(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))
	err := m.Clone(context.Background(), "https://example.com/repo.git", "", true)
	require.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(repoDir, ".git"))
	require.True(t, os.IsNotExist(statErr))
}

func TestPullArgs(t *testing.T) {
	fake := &exec.FakeRunner{}
	m, repoDir := newManager(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))
	err := m.Pull(context.Background())
	require.NoError(t, err)
	require.Equal(t, "git", fake.Calls[0].Name)
	require.Equal(t, []string{"-C", repoDir, "pull", "--ff-only"}, fake.Calls[0].Args)
}

func TestPullNoRepo(t *testing.T) {
	fake := &exec.FakeRunner{}
	m, _ := newManager(t, fake)
	err := m.Pull(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "easyinfra init")
}

func TestPullNonFF(t *testing.T) {
	fake := &exec.FakeRunner{
		Default: exec.FakeResponse{
			Stderr: "fatal: Not possible to fast-forward, aborting.",
			Err:    fmt.Errorf("exit status 128"),
		},
	}
	m, repoDir := newManager(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))
	err := m.Pull(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "fast-forward")
}

func TestPullGenericError(t *testing.T) {
	fake := &exec.FakeRunner{
		Default: exec.FakeResponse{
			Stderr: "fatal: unable to access",
			Err:    fmt.Errorf("exit status 128"),
		},
	}
	m, repoDir := newManager(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))
	err := m.Pull(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "git pull failed")
}

func TestExistsTrue(t *testing.T) {
	fake := &exec.FakeRunner{}
	m, repoDir := newManager(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))
	require.True(t, m.Exists())
}

func TestExistsFalse(t *testing.T) {
	fake := &exec.FakeRunner{}
	m, _ := newManager(t, fake)
	require.False(t, m.Exists())
}

func TestStatus(t *testing.T) {
	fake := &exec.FakeRunner{}
	m, repoDir := newManager(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))
	fake.Responses = map[string]exec.FakeResponse{
		"git -C " + repoDir + " rev-parse HEAD":          {Stdout: "abc123def456"},
		"git -C " + repoDir + " remote get-url origin":   {Stdout: "https://example.com/repo.git"},
	}
	status, err := m.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, "abc123def456", status.Commit)
	require.Equal(t, "https://example.com/repo.git", status.Origin)
}

func TestStatusNoRepo(t *testing.T) {
	fake := &exec.FakeRunner{}
	m, _ := newManager(t, fake)
	_, err := m.Status(context.Background())
	require.Error(t, err)
}

func TestStatusRevParseError(t *testing.T) {
	fake := &exec.FakeRunner{
		Default: exec.FakeResponse{Err: fmt.Errorf("boom")},
	}
	m, repoDir := newManager(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))
	_, err := m.Status(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "getting HEAD")
}
