package k8s

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/guneet/easyinfra/pkg/exec"
)

func newFake(stdout string, err error) *exec.FakeRunner {
	return &exec.FakeRunner{
		Default: exec.FakeResponse{Stdout: stdout, Err: err},
	}
}

func TestCurrentContext(t *testing.T) {
	f := newFake("my-ctx\n", nil)
	c := &Client{Runner: f}
	got, err := c.CurrentContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "my-ctx" {
		t.Errorf("got %q, want %q", got, "my-ctx")
	}
	if len(f.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.Calls))
	}
	if f.Calls[0].Name != "kubectl" {
		t.Errorf("expected kubectl, got %q", f.Calls[0].Name)
	}
	got2 := strings.Join(f.Calls[0].Args, " ")
	if !strings.Contains(got2, "config current-context") {
		t.Errorf("expected 'config current-context' in args, got %q", got2)
	}
}

func TestGetPVCVolumeName(t *testing.T) {
	f := newFake("pv-abc", nil)
	c := &Client{Runner: f}
	got, err := c.GetPVCVolumeName(context.Background(), "ns", "data-pvc")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "pv-abc" {
		t.Errorf("got %q, want pv-abc", got)
	}
	args := strings.Join(f.Calls[0].Args, " ")
	if !strings.Contains(args, "get pvc data-pvc -n ns") {
		t.Errorf("expected get pvc args, got %q", args)
	}
}

func TestGetPVCVolumeNameEmpty(t *testing.T) {
	f := newFake("", nil)
	c := &Client{Runner: f}
	_, err := c.GetPVCVolumeName(context.Background(), "ns", "data-pvc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no bound volume") {
		t.Errorf("expected 'no bound volume' in error, got %q", err.Error())
	}
}

func TestGetPVCVolumeNameError(t *testing.T) {
	f := newFake("", errors.New("kubectl failure"))
	c := &Client{Runner: f}
	_, err := c.GetPVCVolumeName(context.Background(), "ns", "data-pvc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in err, got %q", err.Error())
	}
}

func TestGetPVLocalPath(t *testing.T) {
	f := newFake("/var/lib/data", nil)
	c := &Client{Runner: f}
	got, err := c.GetPVLocalPath(context.Background(), "pv-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "/var/lib/data" {
		t.Errorf("got %q, want /var/lib/data", got)
	}
}

func TestGetPVLocalPathEmpty(t *testing.T) {
	f := newFake("", nil)
	c := &Client{Runner: f}
	_, err := c.GetPVLocalPath(context.Background(), "pv-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not a local PV") {
		t.Errorf("expected 'not a local PV' in err, got %q", err.Error())
	}
}

func TestGetPVLocalPathError(t *testing.T) {
	f := newFake("", errors.New("boom"))
	c := &Client{Runner: f}
	_, err := c.GetPVLocalPath(context.Background(), "pv-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListDeployments(t *testing.T) {
	f := newFake("dep-a dep-b", nil)
	c := &Client{Runner: f}
	got, err := c.ListDeployments(context.Background(), "ns")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 2 || got[0] != "dep-a" || got[1] != "dep-b" {
		t.Errorf("got %v, want [dep-a dep-b]", got)
	}
}

func TestListDeploymentsEmpty(t *testing.T) {
	f := newFake("", nil)
	c := &Client{Runner: f}
	got, err := c.ListDeployments(context.Background(), "ns")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestListDeploymentsError(t *testing.T) {
	f := newFake("", errors.New("boom"))
	c := &Client{Runner: f}
	_, err := c.ListDeployments(context.Background(), "ns")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetDeploymentReplicas(t *testing.T) {
	f := newFake("3", nil)
	c := &Client{Runner: f}
	got, err := c.GetDeploymentReplicas(context.Background(), "ns", "dep")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 3 {
		t.Errorf("got %d, want 3", got)
	}
}

func TestGetDeploymentReplicasEmpty(t *testing.T) {
	f := newFake("", nil)
	c := &Client{Runner: f}
	got, err := c.GetDeploymentReplicas(context.Background(), "ns", "dep")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestGetDeploymentReplicasInvalid(t *testing.T) {
	f := newFake("notanumber", nil)
	c := &Client{Runner: f}
	_, err := c.GetDeploymentReplicas(context.Background(), "ns", "dep")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGetDeploymentReplicasError(t *testing.T) {
	f := newFake("", errors.New("boom"))
	c := &Client{Runner: f}
	_, err := c.GetDeploymentReplicas(context.Background(), "ns", "dep")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestScaleDeployment(t *testing.T) {
	f := newFake("", nil)
	c := &Client{Runner: f}
	if err := c.ScaleDeployment(context.Background(), "ns", "dep", 2); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	args := strings.Join(f.Calls[0].Args, " ")
	for _, want := range []string{"scale", "deployment", "dep", "--replicas=2"} {
		if !strings.Contains(args, want) {
			t.Errorf("expected %q in args, got %q", want, args)
		}
	}
}

func TestScaleDeploymentError(t *testing.T) {
	f := newFake("", errors.New("boom"))
	c := &Client{Runner: f}
	if err := c.ScaleDeployment(context.Background(), "ns", "dep", 2); err == nil {
		t.Fatal("expected error")
	}
}

func TestWaitForPodsDeleted(t *testing.T) {
	f := newFake("", nil)
	c := &Client{Runner: f}
	if err := c.WaitForPodsDeleted(context.Background(), "ns"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	args := strings.Join(f.Calls[0].Args, " ")
	for _, want := range []string{"wait", "--for=delete", "pod", "--all"} {
		if !strings.Contains(args, want) {
			t.Errorf("expected %q in args, got %q", want, args)
		}
	}
}

func TestWaitForPodsDeletedSwallowsError(t *testing.T) {
	f := newFake("", errors.New("no pods"))
	c := &Client{Runner: f}
	if err := c.WaitForPodsDeleted(context.Background(), "ns"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestGlobalArgsContext(t *testing.T) {
	f := newFake("pv-1", nil)
	c := &Client{Runner: f, Context: "my-ctx"}
	if _, err := c.GetPVCVolumeName(context.Background(), "ns", "pvc"); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(f.Calls[0].Args, " ")
	if !strings.Contains(args, "--context my-ctx") {
		t.Errorf("expected --context my-ctx, got %q", args)
	}
}

func TestGlobalArgsKubeconfig(t *testing.T) {
	f := newFake("pv-1", nil)
	c := &Client{Runner: f, Kubeconfig: "/path/to/kube"}
	if _, err := c.GetPVCVolumeName(context.Background(), "ns", "pvc"); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(f.Calls[0].Args, " ")
	if !strings.Contains(args, "--kubeconfig /path/to/kube") {
		t.Errorf("expected --kubeconfig /path/to/kube, got %q", args)
	}
}

func TestCurrentContextError(t *testing.T) {
	f := newFake("", errors.New("boom"))
	c := &Client{Runner: f}
	if _, err := c.CurrentContext(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
