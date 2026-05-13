// Package postrender validates and probes Helm post-renderer binaries.
package postrender

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/guneet-xyz/easyinfra/pkg/config"
)

// Sentinel errors returned by Validate.
var (
	// ErrBinaryMissing indicates the post-renderer binary was not found on PATH.
	ErrBinaryMissing = errors.New("post-renderer binary not found on PATH")
	// ErrBinaryUnresponsive indicates the binary was found but did not respond to a probe.
	ErrBinaryUnresponsive = errors.New("post-renderer binary did not respond")
)

// probeTimeout is the timeout for the help/version invocation.
const probeTimeout = 3 * time.Second

// Result describes the outcome of probing a post-renderer binary.
type Result struct {
	Found   bool
	Version string
	Path    string
}

// helpArgs returns the arguments used to probe the binary.
func helpArgs(binary string) []string {
	if binary == "obscuro" {
		return []string{"version"}
	}
	return []string{"--help"}
}

// lookup resolves the binary path on PATH.
func lookup(cfg *config.PostRenderer) (string, error) {
	if cfg == nil || cfg.Command == "" {
		return "", ErrBinaryMissing
	}
	path, err := exec.LookPath(cfg.Command)
	if err != nil {
		return "", ErrBinaryMissing
	}
	return path, nil
}

// runProbe executes the help/version subcommand and returns its combined output.
func runProbe(path string, binary string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, helpArgs(binary)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), ErrBinaryUnresponsive
	}
	return string(out), nil
}

// Validate verifies that the post-renderer binary exists on PATH and responds to a probe.
func Validate(cfg *config.PostRenderer) error {
	path, err := lookup(cfg)
	if err != nil {
		return err
	}
	if _, err := runProbe(path, cfg.Command); err != nil {
		return err
	}
	return nil
}

// Probe returns information about the post-renderer binary without erroring.
func Probe(cfg *config.PostRenderer) Result {
	path, err := lookup(cfg)
	if err != nil {
		return Result{Found: false}
	}
	out, err := runProbe(path, cfg.Command)
	if err != nil {
		return Result{Found: true, Path: path}
	}
	return Result{
		Found:   true,
		Path:    path,
		Version: parseVersion(out),
	}
}

// parseVersion extracts a best-effort version string from the first line of output.
func parseVersion(out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	if idx := strings.IndexByte(out, '\n'); idx >= 0 {
		return strings.TrimSpace(out[:idx])
	}
	return out
}
