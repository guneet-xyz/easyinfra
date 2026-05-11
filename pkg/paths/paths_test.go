package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDirDefault(t *testing.T) {
	// Unset XDG_CONFIG_HOME to test default behavior
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	result, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() failed: %v", err)
	}

	if !strings.HasSuffix(result, filepath.Join("easyinfra")) {
		t.Errorf("ConfigDir() = %q, want to end with 'easyinfra'", result)
	}
}

func TestConfigDirXDGOverride(t *testing.T) {
	// Set XDG_CONFIG_HOME to test override
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	xdgTest := "/tmp/xdg-test-easyinfra"
	os.Setenv("XDG_CONFIG_HOME", xdgTest)
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	result, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() failed: %v", err)
	}

	expected := filepath.Join(xdgTest, "easyinfra")
	if result != expected {
		t.Errorf("ConfigDir() = %q, want %q", result, expected)
	}
}

func TestRepoDirEndsWithRepo(t *testing.T) {
	// Unset XDG_CONFIG_HOME to use default
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	result, err := RepoDir()
	if err != nil {
		t.Fatalf("RepoDir() failed: %v", err)
	}

	if !strings.HasSuffix(result, filepath.Join("easyinfra", "repo")) {
		t.Errorf("RepoDir() = %q, want to end with 'easyinfra/repo'", result)
	}
}

func TestDefaultConfigPathEndsWithInfraYaml(t *testing.T) {
	// Unset XDG_CONFIG_HOME to use default
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	result, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() failed: %v", err)
	}

	if !strings.HasSuffix(result, "infra.yaml") {
		t.Errorf("DefaultConfigPath() = %q, want to end with 'infra.yaml'", result)
	}
}

func TestEnsureDir(t *testing.T) {
	// Create a temporary directory for testing
	tempBase := t.TempDir()
	testPath := filepath.Join(tempBase, "test", "nested", "dir")

	// First call should create the directory
	err := EnsureDir(testPath)
	if err != nil {
		t.Fatalf("EnsureDir() failed on first call: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Path exists but is not a directory")
	}

	// Second call should be idempotent (no error)
	err = EnsureDir(testPath)
	if err != nil {
		t.Fatalf("EnsureDir() failed on second call (idempotent): %v", err)
	}

	// Verify directory still exists
	info, err = os.Stat(testPath)
	if err != nil {
		t.Fatalf("Directory was removed: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Path is no longer a directory")
	}
}

func TestBinaryDir(t *testing.T) {
	result, err := BinaryDir()
	if err != nil {
		t.Fatalf("BinaryDir() failed: %v", err)
	}

	if result == "" {
		t.Errorf("BinaryDir() returned empty string")
	}

	info, err := os.Stat(result)
	if err != nil {
		t.Fatalf("BinaryDir() returned non-existent path: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("BinaryDir() returned non-directory path")
	}
}

func TestRepoDirWithXDGOverride(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	xdgTest := "/tmp/xdg-test-repo"
	os.Setenv("XDG_CONFIG_HOME", xdgTest)
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	result, err := RepoDir()
	if err != nil {
		t.Fatalf("RepoDir() failed: %v", err)
	}

	expected := filepath.Join(xdgTest, "easyinfra", "repo")
	if result != expected {
		t.Errorf("RepoDir() = %q, want %q", result, expected)
	}
}

func TestDefaultConfigPathWithXDGOverride(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	xdgTest := "/tmp/xdg-test-config"
	os.Setenv("XDG_CONFIG_HOME", xdgTest)
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	result, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() failed: %v", err)
	}

	expected := filepath.Join(xdgTest, "easyinfra", "repo", "infra.yaml")
	if result != expected {
		t.Errorf("DefaultConfigPath() = %q, want %q", result, expected)
	}
}

func TestConfigDirConsistency(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	cfg1, err1 := ConfigDir()
	cfg2, err2 := ConfigDir()

	if err1 != nil || err2 != nil {
		t.Fatalf("ConfigDir() failed: %v, %v", err1, err2)
	}

	if cfg1 != cfg2 {
		t.Errorf("ConfigDir() not consistent: %q != %q", cfg1, cfg2)
	}
}

func TestPathChaining(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	xdgTest := "/tmp/xdg-chain-test"
	os.Setenv("XDG_CONFIG_HOME", xdgTest)
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	cfg, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() failed: %v", err)
	}

	repo, err := RepoDir()
	if err != nil {
		t.Fatalf("RepoDir() failed: %v", err)
	}

	cfgPath, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() failed: %v", err)
	}

	if !strings.HasPrefix(repo, cfg) {
		t.Errorf("RepoDir() %q should be under ConfigDir() %q", repo, cfg)
	}

	if !strings.HasPrefix(cfgPath, repo) {
		t.Errorf("DefaultConfigPath() %q should be under RepoDir() %q", cfgPath, repo)
	}
}

func TestConfigDirEmptyXDG(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", "")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	result, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() failed: %v", err)
	}

	if !strings.HasSuffix(result, "easyinfra") {
		t.Errorf("ConfigDir() = %q, want to end with 'easyinfra'", result)
	}
}

func TestRepoDirErrorPropagation(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", "/tmp/valid-xdg")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	repo, err := RepoDir()
	if err != nil {
		t.Fatalf("RepoDir() failed: %v", err)
	}

	if !strings.Contains(repo, "repo") {
		t.Errorf("RepoDir() = %q, should contain 'repo'", repo)
	}
}

func TestDefaultConfigPathErrorPropagation(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", "/tmp/valid-xdg-cfg")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	cfgPath, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() failed: %v", err)
	}

	if !strings.Contains(cfgPath, "infra.yaml") {
		t.Errorf("DefaultConfigPath() = %q, should contain 'infra.yaml'", cfgPath)
	}
}

func TestConfigDirWithMockedError(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	oldUserConfigDir := userConfigDir
	userConfigDir = func() (string, error) {
		return "", os.ErrPermission
	}
	defer func() {
		userConfigDir = oldUserConfigDir
	}()

	_, err := ConfigDir()
	if err == nil {
		t.Errorf("ConfigDir() should return error when userConfigDir fails")
	}
	if err != os.ErrPermission {
		t.Errorf("ConfigDir() error = %v, want os.ErrPermission", err)
	}
}

func TestBinaryDirWithMockedError(t *testing.T) {
	oldExecutable := executable
	executable = func() (string, error) {
		return "", os.ErrPermission
	}
	defer func() {
		executable = oldExecutable
	}()

	_, err := BinaryDir()
	if err == nil {
		t.Errorf("BinaryDir() should return error when executable fails")
	}
	if err != os.ErrPermission {
		t.Errorf("BinaryDir() error = %v, want os.ErrPermission", err)
	}
}

func TestRepoDirWithMockedConfigDirError(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	oldUserConfigDir := userConfigDir
	userConfigDir = func() (string, error) {
		return "", os.ErrPermission
	}
	defer func() {
		userConfigDir = oldUserConfigDir
	}()

	_, err := RepoDir()
	if err == nil {
		t.Errorf("RepoDir() should return error when ConfigDir fails")
	}
	if err != os.ErrPermission {
		t.Errorf("RepoDir() error = %v, want os.ErrPermission", err)
	}
}

func TestDefaultConfigPathWithMockedError(t *testing.T) {
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	defer func() {
		if oldXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	oldUserConfigDir := userConfigDir
	userConfigDir = func() (string, error) {
		return "", os.ErrPermission
	}
	defer func() {
		userConfigDir = oldUserConfigDir
	}()

	_, err := DefaultConfigPath()
	if err == nil {
		t.Errorf("DefaultConfigPath() should return error when RepoDir fails")
	}
	if err != os.ErrPermission {
		t.Errorf("DefaultConfigPath() error = %v, want os.ErrPermission", err)
	}
}
