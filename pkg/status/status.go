// Package status provides a typed wrapper around `helm status -o json`.
package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

// ErrNotFound is returned when the release does not exist.
var ErrNotFound = errors.New("release not found")

// ChartInfo describes the chart for a release.
type ChartInfo struct {
	Name    string
	Version string
}

// Release represents a parsed `helm status` result.
type Release struct {
	Name          string
	Namespace     string
	Revision      int
	Status        string
	Updated       time.Time
	Chart         ChartInfo
	FirstDeployed time.Time
	LastDeployed  time.Time
}

// helmStatusJSON mirrors the JSON shape produced by `helm status -o json`.
type helmStatusJSON struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   int    `json:"version"`
	Info      struct {
		FirstDeployed time.Time `json:"first_deployed"`
		LastDeployed  time.Time `json:"last_deployed"`
		Status        string    `json:"status"`
	} `json:"info"`
	Chart struct {
		Metadata struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"metadata"`
	} `json:"chart"`
}

// Status shells `helm status <release> -n <namespace> -o json` and parses the result.
// Returns (nil, ErrNotFound) when the release does not exist.
func Status(ctx context.Context, runner exec.Runner, release, namespace string) (*Release, error) {
	stdout, stderr, err := runner.Run(ctx, "helm", "status", release, "-n", namespace, "-o", "json")
	if err != nil {
		if strings.Contains(stderr, "release: not found") {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("helm status %s: %w\n%s", release, err, stderr)
	}

	var raw helmStatusJSON
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, fmt.Errorf("parse helm status JSON: %w", err)
	}

	return &Release{
		Name:          raw.Name,
		Namespace:     raw.Namespace,
		Revision:      raw.Version,
		Status:        raw.Info.Status,
		Updated:       raw.Info.LastDeployed,
		Chart:         ChartInfo{Name: raw.Chart.Metadata.Name, Version: raw.Chart.Metadata.Version},
		FirstDeployed: raw.Info.FirstDeployed,
		LastDeployed:  raw.Info.LastDeployed,
	}, nil
}
