// Package history provides a typed wrapper around `helm history`.
package history

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

// ErrNotFound is returned when the release does not exist.
var ErrNotFound = errors.New("release not found")

// Revision describes a single helm release revision.
type Revision struct {
	Revision    int       `json:"revision"`
	Updated     time.Time `json:"updated"`
	Status      string    `json:"status"`
	Chart       string    `json:"chart"`
	AppVersion  string    `json:"app_version"`
	Description string    `json:"description"`
}

// helmHistoryJSON mirrors the JSON shape of `helm history -o json`.
type helmHistoryJSON struct {
	Revision    int       `json:"revision"`
	Updated     time.Time `json:"updated"`
	Status      string    `json:"status"`
	Chart       string    `json:"chart"`
	AppVersion  string    `json:"app_version"`
	Description string    `json:"description"`
}

// History shells `helm history <release> -n <namespace> -o json --max <max>`
// and parses the JSON output. Returns revisions sorted by revision number
// descending (newest first). Returns (nil, ErrNotFound) when the release
// does not exist.
func History(ctx context.Context, runner exec.Runner, release, namespace string, max int) ([]Revision, error) {
	if max <= 0 {
		max = 10
	}
	args := []string{"history", release, "-n", namespace, "-o", "json", "--max", strconv.Itoa(max)}
	stdout, stderr, err := runner.Run(ctx, "helm", args...)
	if err != nil {
		if strings.Contains(stderr, "release: not found") {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("helm history %s: %w\n%s", release, err, stderr)
	}

	if strings.TrimSpace(stdout) == "" {
		return nil, nil
	}

	var raw []helmHistoryJSON
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, fmt.Errorf("parse helm history JSON: %w", err)
	}

	out := make([]Revision, 0, len(raw))
	for _, r := range raw {
		out = append(out, Revision{
			Revision:    r.Revision,
			Updated:     r.Updated,
			Status:      r.Status,
			Chart:       r.Chart,
			AppVersion:  r.AppVersion,
			Description: r.Description,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Revision > out[j].Revision
	})
	return out, nil
}
