package helm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

func newClient(fr *exec.FakeRunner) *Client {
	return &Client{Runner: fr}
}

func TestInstallArgs(t *testing.T) {
	fr := &exec.FakeRunner{}
	c := newClient(fr)
	opts := InstallOpts{
		Release:    "caddy",
		Chart:      "apps/caddy",
		Namespace:  "caddy",
		ValueFiles: []string{"values-shared.yaml", "apps/caddy/values.yaml"},
		PostRenderer: &config.PostRenderer{
			Command: "obscuro",
			Args:    []string{"inject"},
		},
	}
	if err := c.Install(context.Background(), opts); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(fr.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fr.Calls))
	}
	expected := []string{
		"install", "caddy", "apps/caddy",
		"-n", "caddy", "--create-namespace",
		"--atomic", "--wait",
		"-f", "values-shared.yaml",
		"-f", "apps/caddy/values.yaml",
		"--post-renderer", "obscuro",
		"--post-renderer-args", "inject",
	}
	if !reflect.DeepEqual(fr.Calls[0].Args, expected) {
		t.Errorf("args mismatch:\ngot:  %v\nwant: %v", fr.Calls[0].Args, expected)
	}
	if fr.Calls[0].Name != "helm" {
		t.Errorf("expected helm, got %s", fr.Calls[0].Name)
	}
}

func TestUpgradeNoCreateNamespace(t *testing.T) {
	fr := &exec.FakeRunner{}
	c := newClient(fr)
	opts := InstallOpts{
		Release:   "x",
		Chart:     "c",
		Namespace: "ns",
	}
	if err := c.Upgrade(context.Background(), opts); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	for _, a := range fr.Calls[0].Args {
		if a == "--create-namespace" {
			t.Fatalf("upgrade must not include --create-namespace; got %v", fr.Calls[0].Args)
		}
	}
}

func TestUninstallArgs(t *testing.T) {
	fr := &exec.FakeRunner{}
	c := newClient(fr)
	if err := c.Uninstall(context.Background(), "myapp", "mynamespace"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	expected := []string{"uninstall", "myapp", "-n", "mynamespace"}
	if !reflect.DeepEqual(fr.Calls[0].Args, expected) {
		t.Errorf("args mismatch:\ngot:  %v\nwant: %v", fr.Calls[0].Args, expected)
	}
}

func TestTemplateArgs(t *testing.T) {
	fr := &exec.FakeRunner{
		Default: exec.FakeResponse{Stdout: "rendered"},
	}
	c := newClient(fr)
	out, err := c.Template(context.Background(), "charts/alpha", []string{"values.yaml"})
	if err != nil {
		t.Fatalf("Template: %v", err)
	}
	if out != "rendered" {
		t.Errorf("expected 'rendered', got %q", out)
	}
	expected := []string{"template", "charts/alpha", "-f", "values.yaml"}
	if !reflect.DeepEqual(fr.Calls[0].Args, expected) {
		t.Errorf("args mismatch:\ngot:  %v\nwant: %v", fr.Calls[0].Args, expected)
	}
}

func TestIsLibraryChart(t *testing.T) {
	dir := t.TempDir()
	chartYaml := filepath.Join(dir, "Chart.yaml")
	content := "apiVersion: v2\nname: mylib\ntype: library\nversion: 0.1.0\n"
	if err := os.WriteFile(chartYaml, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Client{}
	got, err := c.IsLibraryChart(dir)
	if err != nil {
		t.Fatalf("IsLibraryChart: %v", err)
	}
	if !got {
		t.Errorf("expected true for library chart")
	}
}

func TestIsNotLibraryChart(t *testing.T) {
	dir := t.TempDir()
	chartYaml := filepath.Join(dir, "Chart.yaml")
	content := "apiVersion: v2\nname: myapp\ntype: application\nversion: 0.1.0\n"
	if err := os.WriteFile(chartYaml, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Client{}
	got, err := c.IsLibraryChart(dir)
	if err != nil {
		t.Fatalf("IsLibraryChart: %v", err)
	}
	if got {
		t.Errorf("expected false for application chart")
	}
}

func TestIsLibraryChartMissing(t *testing.T) {
	c := &Client{}
	_, err := c.IsLibraryChart("/nonexistent/path/xyz")
	if err == nil {
		t.Fatalf("expected error for missing Chart.yaml")
	}
}

func TestInstallError(t *testing.T) {
	fr := &exec.FakeRunner{
		Default: exec.FakeResponse{
			Stderr: "boom",
			Err:    errors.New("exit 1"),
		},
	}
	c := newClient(fr)
	err := c.Install(context.Background(), InstallOpts{
		Release:   "r",
		Chart:     "c",
		Namespace: "n",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !contains(err.Error(), "helm install") {
		t.Errorf("expected error to contain 'helm install', got %v", err)
	}
}

func TestUpgradeError(t *testing.T) {
	fr := &exec.FakeRunner{
		Default: exec.FakeResponse{Err: errors.New("exit 1")},
	}
	c := newClient(fr)
	err := c.Upgrade(context.Background(), InstallOpts{Release: "r", Chart: "c", Namespace: "n"})
	if err == nil || !contains(err.Error(), "helm upgrade") {
		t.Errorf("expected helm upgrade error, got %v", err)
	}
}

func TestUninstallError(t *testing.T) {
	fr := &exec.FakeRunner{
		Default: exec.FakeResponse{Err: errors.New("exit 1")},
	}
	c := newClient(fr)
	err := c.Uninstall(context.Background(), "r", "n")
	if err == nil || !contains(err.Error(), "helm uninstall") {
		t.Errorf("expected helm uninstall error, got %v", err)
	}
}

func TestTemplateError(t *testing.T) {
	fr := &exec.FakeRunner{
		Default: exec.FakeResponse{Err: errors.New("exit 1")},
	}
	c := newClient(fr)
	_, err := c.Template(context.Background(), "c", nil)
	if err == nil || !contains(err.Error(), "helm template") {
		t.Errorf("expected helm template error, got %v", err)
	}
}

func TestPostRendererNil(t *testing.T) {
	fr := &exec.FakeRunner{}
	c := newClient(fr)
	opts := InstallOpts{
		Release:      "r",
		Chart:        "c",
		Namespace:    "n",
		PostRenderer: nil,
	}
	if err := c.Install(context.Background(), opts); err != nil {
		t.Fatalf("Install: %v", err)
	}
	for _, a := range fr.Calls[0].Args {
		if a == "--post-renderer" {
			t.Fatalf("expected no --post-renderer when nil; got %v", fr.Calls[0].Args)
		}
	}
}

func TestGlobalArgs(t *testing.T) {
	fr := &exec.FakeRunner{}
	c := &Client{
		Runner:     fr,
		Kubeconfig: "/tmp/kc",
		Context:    "prod",
	}
	if err := c.Uninstall(context.Background(), "r", "n"); err != nil {
		t.Fatal(err)
	}
	args := fr.Calls[0].Args
	if len(args) < 4 || args[0] != "--kubeconfig" || args[1] != "/tmp/kc" || args[2] != "--kube-context" || args[3] != "prod" {
		t.Errorf("expected global args first; got %v", args)
	}
}

func TestInstallWithExtraArgs(t *testing.T) {
	fr := &exec.FakeRunner{}
	c := newClient(fr)
	opts := InstallOpts{
		Release:   "r",
		Chart:     "c",
		Namespace: "n",
		ExtraArgs: []string{"--debug", "--timeout", "5m"},
	}
	if err := c.Install(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	args := fr.Calls[0].Args
	last3 := args[len(args)-3:]
	expected := []string{"--debug", "--timeout", "5m"}
	if !reflect.DeepEqual(last3, expected) {
		t.Errorf("expected extra args at end; got %v", args)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
