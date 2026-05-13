// Package config provides configuration management for easyinfra.
package config

import (
	"context"
	"fmt"
	"strings"

	execpkg "github.com/guneet-xyz/easyinfra/pkg/exec"
)

// VerifyKubeContext checks that the current kubectl context matches cfg.KubeContext.
// If force is true, a mismatch is allowed (returns nil).
func VerifyKubeContext(ctx context.Context, cfg *InfraConfig, runner execpkg.Runner, force bool) error {
	stdout, _, err := runner.Run(ctx, "kubectl", "config", "current-context")
	if err != nil {
		return fmt.Errorf("getting current kubectl context: %w", err)
	}
	current := strings.TrimSpace(stdout)
	if current == cfg.KubeContext {
		return nil
	}
	if force {
		return nil
	}
	return fmt.Errorf(
		"kubectl context mismatch: infra.yaml expects %q but current context is %q\n"+
			"Use --confirm-context to proceed anyway, or run: kubectl config use-context %s",
		cfg.KubeContext, current, cfg.KubeContext,
	)
}

// VerifyKubeContextV2 checks that the current kubectl context matches cfg.Cluster.KubeContext.
// If force is true, a mismatch is allowed (returns nil).
func VerifyKubeContextV2(ctx context.Context, cfg *InfraConfigV2, runner execpkg.Runner, force bool) error {
	stdout, _, err := runner.Run(ctx, "kubectl", "config", "current-context")
	if err != nil {
		return fmt.Errorf("getting current kubectl context: %w", err)
	}
	current := strings.TrimSpace(stdout)
	if current == cfg.Cluster.KubeContext {
		return nil
	}
	if force {
		return nil
	}
	return fmt.Errorf(
		"kubectl context mismatch: infra.yaml expects %q but current context is %q\n"+
			"Use --confirm-context to proceed anyway, or run: kubectl config use-context %s",
		cfg.Cluster.KubeContext, current, cfg.Cluster.KubeContext,
	)
}
