// Package backup provides utilities for backing up and restoring PVCs.
package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/guneet-xyz/easyinfra/pkg/k8s"
)

// Manager orchestrates PVC backup and restore operations.
type Manager struct {
	Runner exec.Runner
	K8s    *k8s.Client
	Cfg    config.BackupConfig
}

// replicaState holds saved replica counts for a namespace.
type replicaState struct {
	namespace string
	replicas  map[string]int
}

func (m *Manager) saveReplicas(ctx context.Context, namespace string) (*replicaState, error) {
	deployments, err := m.K8s.ListDeployments(ctx, namespace)
	if err != nil {
		return nil, err
	}
	state := &replicaState{namespace: namespace, replicas: make(map[string]int)}
	for _, dep := range deployments {
		n, err := m.K8s.GetDeploymentReplicas(ctx, namespace, dep)
		if err != nil {
			return nil, err
		}
		state.replicas[dep] = n
	}
	return state, nil
}

func (m *Manager) scaleDown(ctx context.Context, namespace string, deployments []string) error {
	for _, dep := range deployments {
		if err := m.K8s.ScaleDeployment(ctx, namespace, dep, 0); err != nil {
			return err
		}
	}
	return m.K8s.WaitForPodsDeleted(ctx, namespace)
}

func (m *Manager) scaleUp(ctx context.Context, state *replicaState) error {
	// Sort for deterministic ordering.
	names := make([]string, 0, len(state.replicas))
	for n := range state.replicas {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, dep := range names {
		if err := m.K8s.ScaleDeployment(ctx, state.namespace, dep, state.replicas[dep]); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) remoteUser() string {
	if m.Cfg.RemoteUser != "" {
		return m.Cfg.RemoteUser + "@"
	}
	return ""
}

func (m *Manager) remoteTarget() string {
	return m.remoteUser() + m.Cfg.RemoteHost
}

// Run backs up PVCs for the given apps and returns the timestamp used.
func (m *Manager) Run(ctx context.Context, apps []config.AppConfig) (string, error) {
	timestamp := time.Now().Format("2006-01-02_150405")
	remoteTmpTS := m.Cfg.RemoteTmp + "/" + timestamp
	localDir := filepath.Join(m.Cfg.LocalDir, timestamp)

	if err := os.MkdirAll(localDir, 0755); err != nil {
		return "", fmt.Errorf("creating local backup dir %s: %w", localDir, err)
	}

	_, _, err := m.Runner.Run(ctx, "ssh", m.remoteTarget(), "mkdir -p "+remoteTmpTS)
	if err != nil {
		return "", fmt.Errorf("creating remote tmp dir: %w", err)
	}

	var errs []error
	for _, app := range apps {
		if len(app.PVCs) == 0 {
			continue
		}
		if err := m.backupApp(ctx, app, remoteTmpTS, localDir); err != nil {
			errs = append(errs, fmt.Errorf("backup %s: %w", app.Name, err))
		}
	}

	_, _, scpErr := m.Runner.Run(ctx, "scp", "-r",
		m.remoteTarget()+":"+remoteTmpTS+"/*",
		localDir+"/")
	if scpErr != nil {
		errs = append(errs, fmt.Errorf("scp from remote: %w", scpErr))
	}

	_, _, _ = m.Runner.Run(ctx, "ssh", m.remoteTarget(), "rm -rf "+remoteTmpTS)

	if len(errs) > 0 {
		return timestamp, errors.Join(errs...)
	}
	return timestamp, nil
}

func (m *Manager) backupApp(ctx context.Context, app config.AppConfig, remoteTmpTS, localDir string) error {
	state, err := m.saveReplicas(ctx, app.Namespace)
	if err != nil {
		return err
	}

	if err := persistReplicaState(localDir, state); err != nil {
		return fmt.Errorf("persisting replica state: %w", err)
	}

	deployments, err := m.K8s.ListDeployments(ctx, app.Namespace)
	if err != nil {
		return err
	}

	if err := m.scaleDown(ctx, app.Namespace, deployments); err != nil {
		return err
	}

	var tarErrs []error
	for _, pvc := range app.PVCs {
		hostPath, err := m.resolvePVCPath(ctx, app.Namespace, pvc)
		if err != nil {
			tarErrs = append(tarErrs, err)
			continue
		}
		tarCmd := fmt.Sprintf("tar czf %s/%s.tar.gz -C '%s' .", remoteTmpTS, pvc, hostPath)
		_, _, err = m.Runner.Run(ctx, "ssh", m.remoteTarget(), tarCmd)
		if err != nil {
			tarErrs = append(tarErrs, fmt.Errorf("tar %s: %w", pvc, err))
		}
	}

	if err := m.scaleUp(ctx, state); err != nil {
		tarErrs = append(tarErrs, fmt.Errorf("restoring replicas: %w", err))
		if writeErr := writeStateFile(localDir, stateScaleUpFail); writeErr != nil {
			tarErrs = append(tarErrs, fmt.Errorf("writing recovery state file: %w", writeErr))
		}
	}

	return errors.Join(tarErrs...)
}

// persistReplicaState merges (does not overwrite) per-app counts into
// localDir/replicas.json so one file describes all apps in this backup.
func persistReplicaState(localDir string, state *replicaState) error {
	if state == nil {
		return nil
	}
	dir := localDir
	existing, err := readReplicaState(dir)
	if err != nil {
		existing = &ReplicaState{Deployments: map[string]int32{}}
	}
	for name, n := range state.replicas {
		key := state.namespace + "/" + name
		existing.Deployments[key] = int32(n)
	}
	return writeReplicaState(dir, existing)
}

func (m *Manager) resolvePVCPath(ctx context.Context, namespace, pvcName string) (string, error) {
	pvName, err := m.K8s.GetPVCVolumeName(ctx, namespace, pvcName)
	if err != nil {
		return "", err
	}
	return m.K8s.GetPVLocalPath(ctx, pvName)
}

// Restore restores PVCs for the given apps. Empty timestamp uses latest backup.
func (m *Manager) Restore(ctx context.Context, apps []config.AppConfig, timestamp string) error {
	if timestamp == "" {
		ts, err := m.LatestTimestamp()
		if err != nil {
			return err
		}
		timestamp = ts
	}

	localDir := filepath.Join(m.Cfg.LocalDir, timestamp)
	if _, err := os.Stat(localDir); err != nil {
		return fmt.Errorf("backup directory %s does not exist", localDir)
	}

	remoteTmpTS := m.Cfg.RemoteTmp + "/" + timestamp
	_, _, err := m.Runner.Run(ctx, "ssh", m.remoteTarget(), "mkdir -p "+remoteTmpTS)
	if err != nil {
		return fmt.Errorf("creating remote tmp dir: %w", err)
	}

	var errs []error
	for _, app := range apps {
		if len(app.PVCs) == 0 {
			continue
		}
		if err := m.restoreApp(ctx, app, localDir, remoteTmpTS); err != nil {
			errs = append(errs, fmt.Errorf("restore %s: %w", app.Name, err))
		}
	}

	_, _, _ = m.Runner.Run(ctx, "ssh", m.remoteTarget(), "rm -rf "+remoteTmpTS)

	return errors.Join(errs...)
}

func (m *Manager) restoreApp(ctx context.Context, app config.AppConfig, localDir, remoteTmpTS string) error {
	state, err := m.saveReplicas(ctx, app.Namespace)
	if err != nil {
		return err
	}

	deployments, err := m.K8s.ListDeployments(ctx, app.Namespace)
	if err != nil {
		return err
	}

	if err := m.scaleDown(ctx, app.Namespace, deployments); err != nil {
		return err
	}

	var restoreErrs []error
	for _, pvc := range app.PVCs {
		tarFile := filepath.Join(localDir, pvc+".tar.gz")
		_, _, err := m.Runner.Run(ctx, "scp", tarFile,
			m.remoteTarget()+":"+remoteTmpTS+"/"+pvc+".tar.gz")
		if err != nil {
			restoreErrs = append(restoreErrs, fmt.Errorf("scp %s: %w", pvc, err))
			continue
		}

		hostPath, err := m.resolvePVCPath(ctx, app.Namespace, pvc)
		if err != nil {
			restoreErrs = append(restoreErrs, err)
			continue
		}

		extractCmd := fmt.Sprintf("rm -rf '%s'/* && tar xzf %s/%s.tar.gz -C '%s'",
			hostPath, remoteTmpTS, pvc, hostPath)
		_, _, err = m.Runner.Run(ctx, "ssh", m.remoteTarget(), extractCmd)
		if err != nil {
			restoreErrs = append(restoreErrs, fmt.Errorf("extract %s: %w", pvc, err))
		}
	}

	if err := m.scaleUp(ctx, state); err != nil {
		restoreErrs = append(restoreErrs, fmt.Errorf("restoring replicas: %w", err))
	}

	return errors.Join(restoreErrs...)
}

// LatestTimestamp returns the most recent backup timestamp from the local backup directory.
func (m *Manager) LatestTimestamp() (string, error) {
	entries, err := os.ReadDir(m.Cfg.LocalDir)
	if err != nil {
		return "", fmt.Errorf("reading backup dir %s: %w", m.Cfg.LocalDir, err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no backups found in %s", m.Cfg.LocalDir)
	}
	sort.Strings(dirs)
	return dirs[len(dirs)-1], nil
}
