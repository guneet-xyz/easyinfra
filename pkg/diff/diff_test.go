package diff

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

func TestDiff_BuildsExpectedArgv(t *testing.T) {
	runner := &exec.FakeRunner{
		Default: exec.FakeResponse{Stdout: "no changes"},
	}
	app := config.AppConfigV2{
		Name:       "walls",
		Chart:      "apps/walls",
		Namespace:  "walls",
		ValueFiles: []string{"values/shared.yaml", "apps/walls/values.yaml"},
	}
	cfg := &config.InfraConfigV2{}

	out, err := Diff(context.Background(), runner, app, cfg, "/repo")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if out != "no changes" {
		t.Fatalf("unexpected stdout: %q", out)
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}
	got := runner.Calls[0]
	if got.Name != "helm" {
		t.Fatalf("expected helm, got %q", got.Name)
	}
	want := []string{
		"diff", "upgrade", "walls", "/repo/apps/walls",
		"-n", "walls",
		"-f", "/repo/values/shared.yaml",
		"-f", "/repo/apps/walls/values.yaml",
	}
	if !reflect.DeepEqual(got.Args, want) {
		t.Fatalf("argv mismatch:\n got:  %v\n want: %v", got.Args, want)
	}
}

func TestDiff_IncludesPostRenderer(t *testing.T) {
	runner := &exec.FakeRunner{Default: exec.FakeResponse{}}
	app := config.AppConfigV2{
		Name:      "walls",
		Chart:     "apps/walls",
		Namespace: "walls",
		PostRenderer: &config.PostRenderer{
			Command: "obscuro",
			Args:    []string{"inject", "--mode=prod"},
		},
	}
	cfg := &config.InfraConfigV2{}

	if _, err := Diff(context.Background(), runner, app, cfg, "/repo"); err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}
	args := runner.Calls[0].Args
	want := []string{
		"diff", "upgrade", "walls", "/repo/apps/walls",
		"-n", "walls",
		"--post-renderer", "obscuro",
		"--post-renderer-args", "inject",
		"--post-renderer-args", "--mode=prod",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("argv mismatch:\n got:  %v\n want: %v", args, want)
	}
}

func TestDiff_PluginMissingReturnsSentinel(t *testing.T) {
	runner := &exec.FakeRunner{
		Default: exec.FakeResponse{
			Stderr: `Error: unknown command "diff" for "helm"`,
			Err:    errors.New("exit status 1"),
		},
	}
	app := config.AppConfigV2{Name: "walls", Chart: "apps/walls", Namespace: "walls"}
	cfg := &config.InfraConfigV2{}

	_, err := Diff(context.Background(), runner, app, cfg, "/repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsPluginMissing(err) {
		t.Fatalf("expected IsPluginMissing(err)=true, got false (err=%v)", err)
	}
	if !errors.Is(err, ErrPluginMissing) {
		t.Fatalf("expected errors.Is(err, ErrPluginMissing)=true, got false (err=%v)", err)
	}
}

func TestIsPluginMissing(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"sentinel", ErrPluginMissing, true},
		{"unknown command", errors.New(`Error: unknown command "diff" for "helm"`), true},
		{"plugin not found quoted", errors.New(`Error: plugin "diff" not found`), true},
		{"plugin not found bare", errors.New(`plugin not found: diff`), true},
		{"unrelated", errors.New("connection refused"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPluginMissing(tc.err); got != tc.want {
				t.Fatalf("IsPluginMissing(%v)=%v want %v", tc.err, got, tc.want)
			}
		})
	}
}
