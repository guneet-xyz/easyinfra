// Package k8s provides Kubernetes client utilities.
package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

// Client wraps kubectl commands.
type Client struct {
	Runner     exec.Runner
	Context    string // --context flag value (empty = use current)
	Kubeconfig string // --kubeconfig flag value (empty = use default)
}

// globalArgs returns common kubectl flags based on client config.
func (c *Client) globalArgs() []string {
	var args []string
	if c.Context != "" {
		args = append(args, "--context", c.Context)
	}
	if c.Kubeconfig != "" {
		args = append(args, "--kubeconfig", c.Kubeconfig)
	}
	return args
}

func (c *Client) run(ctx context.Context, args ...string) (string, string, error) {
	all := append(c.globalArgs(), args...)
	return c.Runner.Run(ctx, "kubectl", all...)
}

// CurrentContext returns the active kubectl context name.
func (c *Client) CurrentContext(ctx context.Context) (string, error) {
	stdout, _, err := c.Runner.Run(ctx, "kubectl", "config", "current-context")
	if err != nil {
		return "", fmt.Errorf("getting current kubectl context: %w", err)
	}
	return strings.TrimSpace(stdout), nil
}

// GetPVCVolumeName returns the PV name bound to a PVC.
func (c *Client) GetPVCVolumeName(ctx context.Context, namespace, pvcName string) (string, error) {
	stdout, _, err := c.run(ctx, "get", "pvc", pvcName, "-n", namespace,
		"-o", "jsonpath={.spec.volumeName}")
	if err != nil {
		return "", fmt.Errorf("PVC %q not found in namespace %q: %w", pvcName, namespace, err)
	}
	vol := strings.TrimSpace(stdout)
	if vol == "" {
		return "", fmt.Errorf("PVC %q in namespace %q has no bound volume", pvcName, namespace)
	}
	return vol, nil
}

// GetPVLocalPath returns the local.path of a PersistentVolume.
// Returns an error if the PV is not a local PV.
func (c *Client) GetPVLocalPath(ctx context.Context, pvName string) (string, error) {
	stdout, _, err := c.run(ctx, "get", "pv", pvName,
		"-o", "jsonpath={.spec.local.path}")
	if err != nil {
		return "", fmt.Errorf("getting PV %q: %w", pvName, err)
	}
	path := strings.TrimSpace(stdout)
	if path == "" {
		return "", fmt.Errorf("PV %q is not a local PV (spec.local.path is empty); only local PVs are supported for backup", pvName)
	}
	return path, nil
}

// ListDeployments returns the names of all deployments in a namespace.
func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]string, error) {
	stdout, _, err := c.run(ctx, "get", "deployments", "-n", namespace,
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, fmt.Errorf("listing deployments in %q: %w", namespace, err)
	}
	names := strings.Fields(strings.TrimSpace(stdout))
	return names, nil
}

// GetDeploymentReplicas returns the current replica count for a deployment.
func (c *Client) GetDeploymentReplicas(ctx context.Context, namespace, name string) (int, error) {
	stdout, _, err := c.run(ctx, "get", "deployment", name, "-n", namespace,
		"-o", "jsonpath={.spec.replicas}")
	if err != nil {
		return 0, fmt.Errorf("getting replicas for %s/%s: %w", namespace, name, err)
	}
	val := strings.TrimSpace(stdout)
	if val == "" {
		return 0, nil
	}
	var n int
	if _, err := fmt.Sscanf(val, "%d", &n); err != nil {
		return 0, fmt.Errorf("parsing replica count %q: %w", val, err)
	}
	return n, nil
}

// ScaleDeployment scales a deployment to the given replica count.
func (c *Client) ScaleDeployment(ctx context.Context, namespace, name string, replicas int) error {
	_, _, err := c.run(ctx, "scale", "deployment", name,
		"-n", namespace,
		fmt.Sprintf("--replicas=%d", replicas),
		"--timeout=120s")
	if err != nil {
		return fmt.Errorf("scaling %s/%s to %d: %w", namespace, name, replicas, err)
	}
	return nil
}

// WaitForPodsDeleted waits for all pods in a namespace to be deleted.
func (c *Client) WaitForPodsDeleted(ctx context.Context, namespace string) error {
	_, _, err := c.run(ctx, "wait", "--for=delete", "pod", "--all",
		"-n", namespace, "--timeout=120s")
	if err != nil {
		// kubectl wait exits non-zero if no pods exist — that's fine.
		return nil
	}
	return nil
}
