package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BackupEntry describes a single timestamped backup directory.
type BackupEntry struct {
	Timestamp   string   `json:"timestamp"`
	Apps        []string `json:"apps"`
	SizeBytes   int64    `json:"sizeBytes"`
	HasReplicas bool     `json:"hasReplicas"`
}

// List enumerates backup directories under localDir and returns one
// BackupEntry per timestamped subdirectory, sorted newest-first by
// timestamp. Subdirectories whose names do not match the timestamp
// format "2006-01-02_150405" are skipped.
func List(localDir string) ([]BackupEntry, error) {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading backup dir %s: %w", localDir, err)
	}

	var result []BackupEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !looksLikeTimestamp(name) {
			continue
		}
		entry, err := scanBackupDir(filepath.Join(localDir, name), name)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp > result[j].Timestamp
	})
	return result, nil
}

// looksLikeTimestamp returns true if name matches the layout
// "YYYY-MM-DD_HHMMSS" produced by Manager.Run. We avoid time.Parse here
// because the validation only needs to be cheap and structural.
func looksLikeTimestamp(name string) bool {
	if len(name) != len("2006-01-02_150405") {
		return false
	}
	for i, r := range name {
		switch i {
		case 4, 7:
			if r != '-' {
				return false
			}
		case 10:
			if r != '_' {
				return false
			}
		default:
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func scanBackupDir(dir, ts string) (BackupEntry, error) {
	entry := BackupEntry{Timestamp: ts}

	files, err := os.ReadDir(dir)
	if err != nil {
		return entry, fmt.Errorf("reading %s: %w", dir, err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		info, err := f.Info()
		if err != nil {
			return entry, fmt.Errorf("stat %s: %w", filepath.Join(dir, name), err)
		}
		entry.SizeBytes += info.Size()

		switch {
		case name == "replicas.json":
			entry.HasReplicas = true
		case strings.HasSuffix(name, ".tar"):
			entry.Apps = append(entry.Apps, strings.TrimSuffix(name, ".tar"))
		}
	}

	sort.Strings(entry.Apps)
	return entry, nil
}
