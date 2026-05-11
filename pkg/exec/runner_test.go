package exec

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealRunnerSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := &RealRunner{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	out, errOut, err := runner.Run(context.Background(), "echo", "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
	assert.Equal(t, "", errOut)
}

func TestRealRunnerNonZeroExit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := &RealRunner{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	_, _, err := runner.Run(context.Background(), "sh", "-c", "exit 1")
	assert.Error(t, err)
}

func TestRealRunnerDryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := &RealRunner{
		DryRun: true,
		Stdout: &stdout,
		Stderr: &stderr,
	}

	out, errOut, err := runner.Run(context.Background(), "echo", "hello")
	require.NoError(t, err)
	assert.Equal(t, "", out)
	assert.Equal(t, "", errOut)
	assert.Contains(t, stdout.String(), "would run: echo hello")
}

func TestRealRunnerVerbose(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := &RealRunner{
		Verbose: true,
		Stdout:  &stdout,
		Stderr:  &stderr,
	}

	_, _, err := runner.Run(context.Background(), "echo", "hi")
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "running: echo hi")
}

func TestFakeRunnerRecordsCalls(t *testing.T) {
	runner := &FakeRunner{
		Responses: make(map[string]FakeResponse),
	}

	_, _, _ = runner.Run(context.Background(), "echo", "hello")
	_, _, _ = runner.Run(context.Background(), "ls", "-la")

	require.Len(t, runner.Calls, 2)
	assert.Equal(t, "echo", runner.Calls[0].Name)
	assert.Equal(t, []string{"hello"}, runner.Calls[0].Args)
	assert.Equal(t, "ls", runner.Calls[1].Name)
	assert.Equal(t, []string{"-la"}, runner.Calls[1].Args)
}

func TestFakeRunnerCannedResponse(t *testing.T) {
	runner := &FakeRunner{
		Responses: map[string]FakeResponse{
			"echo hello": {
				Stdout: "world",
				Stderr: "",
				Err:    nil,
			},
		},
	}

	out, errOut, err := runner.Run(context.Background(), "echo", "hello")
	require.NoError(t, err)
	assert.Equal(t, "world", out)
	assert.Equal(t, "", errOut)
}

func TestFakeRunnerDefault(t *testing.T) {
	runner := &FakeRunner{
		Responses: make(map[string]FakeResponse),
		Default: FakeResponse{
			Stdout: "default out",
			Stderr: "default err",
			Err:    errors.New("default error"),
		},
	}

	out, errOut, err := runner.Run(context.Background(), "unknown", "cmd")
	assert.Equal(t, "default out", out)
	assert.Equal(t, "default err", errOut)
	assert.Error(t, err)
	assert.Equal(t, "default error", err.Error())
}

func TestRealRunnerRunInteractiveDryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := &RealRunner{
		DryRun: true,
		Stdout: &stdout,
		Stderr: &stderr,
	}

	err := runner.RunInteractive(context.Background(), "echo", "test")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "would run: echo test")
}

func TestRealRunnerRunInteractiveSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := &RealRunner{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	err := runner.RunInteractive(context.Background(), "echo", "interactive")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "interactive")
}

func TestFakeRunnerRunInteractive(t *testing.T) {
	runner := &FakeRunner{
		Responses: map[string]FakeResponse{
			"test cmd": {
				Stdout: "output",
				Stderr: "",
				Err:    nil,
			},
		},
	}

	err := runner.RunInteractive(context.Background(), "test", "cmd")
	require.NoError(t, err)
	require.Len(t, runner.Calls, 1)
	assert.Equal(t, "test", runner.Calls[0].Name)
}
