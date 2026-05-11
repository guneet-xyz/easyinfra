package k3s

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func setupFakeRunner(t *testing.T, fake *exec.FakeRunner) {
	t.Helper()
	orig := newRunner
	newRunner = func(_ *cobra.Command, _ *RootFlags) exec.Runner { return fake }
	t.Cleanup(func() { newRunner = orig })
}

func makeFixture(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "infra"))
	require.NoError(t, err)
	dst := t.TempDir()
	require.NoError(t, copyTree(src, dst))
	return dst
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

func newK3sRoot() *cobra.Command {
	root := &cobra.Command{Use: "easyinfra"}
	flags := &RootFlags{}
	root.PersistentFlags().StringVar(&flags.Config, "config", "", "")
	root.PersistentFlags().BoolVar(&flags.DryRun, "dry-run", false, "")
	root.PersistentFlags().BoolVar(&flags.Verbose, "verbose", false, "")
	root.PersistentFlags().BoolVar(&flags.ConfirmContext, "confirm-context", false, "")
	root.AddCommand(NewK3sCmd(flags))
	return root
}

func TestValidateAllChartsPass(t *testing.T) {
	fake := &exec.FakeRunner{Default: exec.FakeResponse{Stdout: "rendered: yaml"}}
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out, errb bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"k3s", "validate", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	stdout := out.String()
	require.Contains(t, stdout, "OK:   alpha")
	require.Contains(t, stdout, "OK:   beta")
	require.Contains(t, stdout, "2/2 charts validated")

	helmCalls := 0
	for _, c := range fake.Calls {
		if c.Name == "helm" {
			helmCalls++
		}
	}
	require.Equal(t, 2, helmCalls)
}

func TestValidateFiltersByAppName(t *testing.T) {
	fake := &exec.FakeRunner{Default: exec.FakeResponse{Stdout: "ok"}}
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "validate", "--config", cfgPath, "alpha"})

	require.NoError(t, cmd.Execute())

	stdout := out.String()
	require.Contains(t, stdout, "OK:   alpha")
	require.NotContains(t, stdout, "beta")
	require.Contains(t, stdout, "1/1 charts validated")
}

func TestValidateLibraryChartSkipped(t *testing.T) {
	fake := &exec.FakeRunner{Default: exec.FakeResponse{Stdout: "ok"}}
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	betaChart := filepath.Join(fixture, "charts", "beta", "Chart.yaml")
	require.NoError(t, os.WriteFile(betaChart, []byte("apiVersion: v2\nname: beta\ntype: library\nversion: 0.1.0\n"), 0o644))

	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "validate", "--config", cfgPath})

	require.NoError(t, cmd.Execute())

	stdout := out.String()
	require.Contains(t, stdout, "OK:   alpha")
	require.Contains(t, stdout, "SKIP: beta")
	require.Contains(t, stdout, "library")
	require.Contains(t, stdout, "1/1 charts validated")

	helmCalls := 0
	for _, c := range fake.Calls {
		if c.Name == "helm" {
			helmCalls++
		}
	}
	require.Equal(t, 1, helmCalls)
}

func TestValidateInvalidChartFails(t *testing.T) {
	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	betaChart := filepath.Join(fixture, "charts", "beta")
	fake := &exec.FakeRunner{Default: exec.FakeResponse{Stdout: "ok"}}

	orig := newRunner
	newRunner = func(_ *cobra.Command, _ *RootFlags) exec.Runner {
		return &chartFailRunner{failChart: betaChart, inner: fake}
	}
	t.Cleanup(func() { newRunner = orig })

	var out bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"k3s", "validate", "--config", cfgPath})

	err := cmd.Execute()
	require.Error(t, err)

	stdout := out.String()
	require.Contains(t, stdout, "OK:   alpha")
	require.Contains(t, stdout, "FAIL: beta")
	require.Contains(t, stdout, "1/2 charts validated")
}

func TestValidateUnknownAppErrors(t *testing.T) {
	fake := &exec.FakeRunner{Default: exec.FakeResponse{Stdout: "ok"}}
	setupFakeRunner(t, fake)

	fixture := makeFixture(t)
	cfgPath := filepath.Join(fixture, "infra.yaml")

	var out, errb bytes.Buffer
	cmd := newK3sRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"k3s", "validate", "--config", cfgPath, "ghost"})

	err := cmd.Execute()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "ghost") || strings.Contains(errb.String(), "ghost"))
}

type chartFailRunner struct {
	failChart string
	inner     *exec.FakeRunner
}

func (r *chartFailRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	for _, a := range args {
		if a == r.failChart {
			return "", "boom", errors.New("template error")
		}
	}
	return r.inner.Run(ctx, name, args...)
}

func (r *chartFailRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	return r.inner.RunInteractive(ctx, name, args...)
}
