package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/stretchr/testify/require"
)

func TestUpdatePullsAndPrintsHEAD(t *testing.T) {
	fake := &exec.FakeRunner{
		Responses: map[string]exec.FakeResponse{},
	}
	repoDir := withFakeRunner(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))

	fake.Responses["git -C "+repoDir+" pull --ff-only"] = exec.FakeResponse{}
	fake.Responses["git -C "+repoDir+" rev-parse HEAD"] = exec.FakeResponse{Stdout: "abc123def"}
	fake.Responses["git -C "+repoDir+" config --get remote.origin.url"] = exec.FakeResponse{Stdout: "https://example.com/r.git"}
	fake.Responses["git -C "+repoDir+" status --porcelain"] = exec.FakeResponse{}

	var out bytes.Buffer
	cmd := newRootCmd("dev", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update"})
	err := cmd.Execute()
	require.NoError(t, err)

	require.Equal(t, []string{"-C", repoDir, "pull", "--ff-only"}, fake.Calls[0].Args)
	require.Contains(t, out.String(), "Updated "+repoDir)
	require.Contains(t, out.String(), "abc123def")
}

func TestUpdateMissingRepo(t *testing.T) {
	fake := &exec.FakeRunner{}
	repoDir := withFakeRunner(t, fake)

	var out bytes.Buffer
	cmd := newRootCmd("dev", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no infra repo at "+repoDir)
	require.Contains(t, err.Error(), "easyinfra init")
}

func TestUpdatePullFailure(t *testing.T) {
	fake := &exec.FakeRunner{
		Default: exec.FakeResponse{Stderr: "fatal: boom", Err: fmt.Errorf("exit 1")},
	}
	repoDir := withFakeRunner(t, fake)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))

	var out bytes.Buffer
	cmd := newRootCmd("dev", "x", "x")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update"})
	err := cmd.Execute()
	require.Error(t, err)
}
