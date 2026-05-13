package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/guneet-xyz/easyinfra/pkg/k8s"
)

// ReplicaState is the on-disk snapshot of deployment replica counts captured
// just before a backup scales workloads down. It is keyed by
// "<namespace>/<name>" so a single file can describe multiple apps.
type ReplicaState struct {
	Deployments map[string]int32 `json:"deployments"`
}

const (
	replicasFileName  = "replicas.json"
	stateFileName     = "state"
	stateScaleUpFail  = "scale-up-failed"
)

func writeReplicaState(dir string, state *ReplicaState) error {
	if state == nil {
		state = &ReplicaState{Deployments: map[string]int32{}}
	}
	if state.Deployments == nil {
		state.Deployments = map[string]int32{}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, replicasFileName)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling replica state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming %s: %w", tmp, err)
	}
	return nil
}

func readReplicaState(dir string) (*ReplicaState, error) {
	path := filepath.Join(dir, replicasFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var state ReplicaState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if state.Deployments == nil {
		state.Deployments = map[string]int32{}
	}
	return &state, nil
}

func writeStateFile(dir, content string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, stateFileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Recover reads the persisted replica state from <localDir>/<ts>/replicas.json
// and re-applies the recorded replica counts by issuing scale commands via the
// supplied runner. Optional appFilter limits recovery to deployments whose
// namespace matches one of the given app namespaces (the typical mapping in
// this project is namespace == app name).
//
// On success, any "<localDir>/<ts>/state" marker file is removed.
func Recover(ctx context.Context, localDir, ts string, runner exec.Runner, appFilter ...string) error {
	if runner == nil {
		return errors.New("recover: runner is nil")
	}
	if ts == "" {
		return errors.New("recover: timestamp is required")
	}
	dir := filepath.Join(localDir, ts)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("backup directory %s does not exist: %w", dir, err)
	}

	state, err := readReplicaState(dir)
	if err != nil {
		return err
	}

	filter := make(map[string]bool, len(appFilter))
	for _, n := range appFilter {
		if n != "" {
			filter[n] = true
		}
	}

	keys := make([]string, 0, len(state.Deployments))
	for k := range state.Deployments {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	client := &k8s.Client{Runner: runner}

	var errs []error
	for _, key := range keys {
		ns, name, ok := splitNSName(key)
		if !ok {
			errs = append(errs, fmt.Errorf("recover: invalid deployment key %q (want namespace/name)", key))
			continue
		}
		if len(filter) > 0 && !filter[ns] {
			continue
		}
		replicas := int(state.Deployments[key])
		if err := client.ScaleDeployment(ctx, ns, name, replicas); err != nil {
			errs = append(errs, fmt.Errorf("recover %s: %w", key, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	// Clear the state marker if present — recovery succeeded.
	_ = os.Remove(filepath.Join(dir, stateFileName))
	return nil
}

func splitNSName(key string) (namespace, name string, ok bool) {
	i := strings.Index(key, "/")
	if i <= 0 || i == len(key)-1 {
		return "", "", false
	}
	return key[:i], key[i+1:], true
}
