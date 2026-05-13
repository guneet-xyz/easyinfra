// Package helm provides Helm chart management utilities.
package helm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

// Client wraps helm commands.
type Client struct {
	Runner     exec.Runner
	Kubeconfig string
	Context    string
}

// InstallOpts holds options for helm install/upgrade.
type InstallOpts struct {
	Release      string
	Chart        string // path to chart directory
	Namespace    string
	ValueFiles   []string
	PostRenderer *config.PostRenderer
	ExtraArgs    []string
}

func (c *Client) globalArgs() []string {
	var args []string
	if c.Kubeconfig != "" {
		args = append(args, "--kubeconfig", c.Kubeconfig)
	}
	if c.Context != "" {
		args = append(args, "--kube-context", c.Context)
	}
	return args
}

func (c *Client) buildValueFileArgs(valueFiles []string) []string {
	var args []string
	for _, vf := range valueFiles {
		args = append(args, "-f", vf)
	}
	return args
}

func (c *Client) buildPostRendererArgs(pr *config.PostRenderer) []string {
	if pr == nil {
		return nil
	}
	args := []string{"--post-renderer", pr.Command}
	for _, a := range pr.Args {
		args = append(args, "--post-renderer-args", a)
	}
	return args
}

// Install runs helm install with --atomic --wait.
func (c *Client) Install(ctx context.Context, opts InstallOpts) error {
	args := append(c.globalArgs(), "install", opts.Release, opts.Chart,
		"-n", opts.Namespace, "--create-namespace",
		"--atomic", "--wait")
	args = append(args, c.buildValueFileArgs(opts.ValueFiles)...)
	args = append(args, c.buildPostRendererArgs(opts.PostRenderer)...)
	args = append(args, opts.ExtraArgs...)

	_, stderr, err := c.Runner.Run(ctx, "helm", args...)
	if err != nil {
		return fmt.Errorf("helm install %s: %w\n%s", opts.Release, err, stderr)
	}
	return nil
}

// Upgrade runs helm upgrade with --atomic --wait.
func (c *Client) Upgrade(ctx context.Context, opts InstallOpts) error {
	args := append(c.globalArgs(), "upgrade", opts.Release, opts.Chart,
		"-n", opts.Namespace,
		"--atomic", "--wait")
	args = append(args, c.buildValueFileArgs(opts.ValueFiles)...)
	args = append(args, c.buildPostRendererArgs(opts.PostRenderer)...)
	args = append(args, opts.ExtraArgs...)

	_, stderr, err := c.Runner.Run(ctx, "helm", args...)
	if err != nil {
		return fmt.Errorf("helm upgrade %s: %w\n%s", opts.Release, err, stderr)
	}
	return nil
}

// Uninstall runs helm uninstall.
func (c *Client) Uninstall(ctx context.Context, release, namespace string) error {
	args := append(c.globalArgs(), "uninstall", release, "-n", namespace)
	_, stderr, err := c.Runner.Run(ctx, "helm", args...)
	if err != nil {
		return fmt.Errorf("helm uninstall %s: %w\n%s", release, err, stderr)
	}
	return nil
}

// Rollback runs helm rollback for the given release.
// If revision is 0, the revision argument is omitted and helm rolls back
// to the previous revision.
func (c *Client) Rollback(ctx context.Context, release, namespace string, revision int, force, wait bool) error {
	args := append(c.globalArgs(), "rollback", release)
	if revision > 0 {
		args = append(args, fmt.Sprintf("%d", revision))
	}
	args = append(args, "-n", namespace)
	if force {
		args = append(args, "--force")
	}
	if wait {
		args = append(args, "--wait")
	}
	_, stderr, err := c.Runner.Run(ctx, "helm", args...)
	if err != nil {
		return fmt.Errorf("helm rollback %s: %w\n%s", release, err, stderr)
	}
	return nil
}

// Template runs helm template and returns the rendered YAML.
func (c *Client) Template(ctx context.Context, chart string, valueFiles []string) (string, error) {
	return c.TemplateWithPostRenderer(ctx, chart, valueFiles, nil)
}

// TemplateWithPostRenderer runs helm template with an optional post-renderer.
// If pr is nil, no --post-renderer flag is passed.
func (c *Client) TemplateWithPostRenderer(ctx context.Context, chart string, valueFiles []string, pr *config.PostRenderer) (string, error) {
	args := append(c.globalArgs(), "template", chart)
	args = append(args, c.buildValueFileArgs(valueFiles)...)
	args = append(args, c.buildPostRendererArgs(pr)...)
	stdout, stderr, err := c.Runner.Run(ctx, "helm", args...)
	if err != nil {
		return "", fmt.Errorf("helm template %s: %w\n%s", chart, err, stderr)
	}
	return stdout, nil
}

// IsLibraryChart returns true if the chart at chartPath has type: library in Chart.yaml.
func (c *Client) IsLibraryChart(chartPath string) (bool, error) {
	chartYaml := chartPath + "/Chart.yaml"
	data, err := os.ReadFile(chartYaml)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", chartYaml, err)
	}
	return strings.Contains(string(data), "type: library"), nil
}
