// Package exec provides command execution utilities.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Runner executes external commands.
type Runner interface {
	// Run executes a command and captures stdout/stderr.
	Run(ctx context.Context, name string, args ...string) (stdout, stderr string, err error)
	// RunInteractive executes a command streaming stdout/stderr to the configured writers.
	RunInteractive(ctx context.Context, name string, args ...string) error
}

// RealRunner executes commands for real.
type RealRunner struct {
	DryRun  bool
	Verbose bool
	Stdout  io.Writer
	Stderr  io.Writer
	Env     []string // additional env vars appended to os.Environ()
}

// Run executes a command and captures stdout/stderr.
func (r *RealRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	cmdStr := name + " " + strings.Join(args, " ")
	if r.DryRun {
		_, _ = fmt.Fprintf(r.Stdout, "would run: %s\n", cmdStr)
		return "", "", nil
	}
	if r.Verbose {
		_, _ = fmt.Fprintf(r.Stderr, "running: %s\n", cmdStr)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if len(r.Env) > 0 {
		cmd.Env = append(os.Environ(), r.Env...)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()), err
}

// RunInteractive executes a command streaming stdout/stderr to the configured writers.
func (r *RealRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	cmdStr := name + " " + strings.Join(args, " ")
	if r.DryRun {
		_, _ = fmt.Fprintf(r.Stdout, "would run: %s\n", cmdStr)
		return nil
	}
	if r.Verbose {
		_, _ = fmt.Fprintf(r.Stderr, "running: %s\n", cmdStr)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if len(r.Env) > 0 {
		cmd.Env = append(os.Environ(), r.Env...)
	}
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	return cmd.Run()
}

// FakeCall records a single command invocation.
type FakeCall struct {
	Name string
	Args []string
}

// FakeResponse is the canned response for a command.
type FakeResponse struct {
	Stdout string
	Stderr string
	Err    error
}

// FakeRunner records calls and returns canned responses. For tests only.
type FakeRunner struct {
	Calls     []FakeCall
	Responses map[string]FakeResponse // key: "name arg1 arg2..."
	Default   FakeResponse            // returned when no key matches
}

func (f *FakeRunner) key(name string, args []string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

// Run executes a command and returns the canned response.
func (f *FakeRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	f.Calls = append(f.Calls, FakeCall{Name: name, Args: args})
	if resp, ok := f.Responses[f.key(name, args)]; ok {
		return resp.Stdout, resp.Stderr, resp.Err
	}
	return f.Default.Stdout, f.Default.Stderr, f.Default.Err
}

// RunInteractive executes a command and returns the error from Run.
func (f *FakeRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	_, _, err := f.Run(ctx, name, args...)
	return err
}
