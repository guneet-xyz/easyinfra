package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// timestampLayout is the layout used for backup directory names.
const timestampLayout = "2006-01-02_150405"

// PrunePolicy controls which backups are eligible for pruning.
//
// If KeepN > 0, only the newest N backups are kept; the rest are pruned.
// If OlderThan > 0, any backup whose timestamp is older than the cutoff
// is pruned.
// When both are set, a backup is pruned if either rule says so, but a
// backup is always kept if it falls within the newest KeepN.
// If DryRun is true, no filesystem changes are made.
type PrunePolicy struct {
	KeepN     int
	OlderThan time.Duration
	DryRun    bool
}

// Prune deletes (or, with DryRun, lists) backup directories under
// localDir that match the given policy. It returns the timestamps of
// the backups that were pruned (or would be pruned, in dry-run mode),
// in newest-first order.
func Prune(localDir string, policy PrunePolicy) ([]string, error) {
	entries, err := List(localDir)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	now := time.Now()

	var toDelete []string
	for i, e := range entries {
		// KeepN: always keep the newest N entries.
		if policy.KeepN > 0 && i < policy.KeepN {
			continue
		}

		shouldDelete := false
		if policy.KeepN > 0 && i >= policy.KeepN {
			shouldDelete = true
		}
		if policy.OlderThan > 0 {
			t, err := time.Parse(timestampLayout, e.Timestamp)
			if err != nil {
				return nil, fmt.Errorf("parsing timestamp %q: %w", e.Timestamp, err)
			}
			if now.Sub(t) > policy.OlderThan {
				shouldDelete = true
			}
		}

		if shouldDelete {
			toDelete = append(toDelete, e.Timestamp)
		}
	}

	if policy.DryRun {
		return toDelete, nil
	}

	for _, ts := range toDelete {
		path := filepath.Join(localDir, ts)
		if err := os.RemoveAll(path); err != nil {
			return toDelete, fmt.Errorf("removing %s: %w", path, err)
		}
	}
	return toDelete, nil
}
