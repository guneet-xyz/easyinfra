package paths

import (
	"os"
	"path/filepath"
)

var (
	userConfigDir = os.UserConfigDir
	executable    = os.Executable
)

// ConfigDir returns the easyinfra config directory.
// Respects $XDG_CONFIG_HOME on Linux/macOS, %AppData% on Windows (via os.UserConfigDir).
func ConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "easyinfra"), nil
	}
	base, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "easyinfra"), nil
}

// RepoDir returns the path where the infra repo is cloned.
func RepoDir() (string, error) {
	cfg, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "repo"), nil
}

// DefaultConfigPath returns the default infra.yaml path inside the cloned repo.
func DefaultConfigPath() (string, error) {
	repo, err := RepoDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(repo, "infra.yaml"), nil
}

// BinaryDir returns the directory containing the current executable.
func BinaryDir() (string, error) {
	exe, err := executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// EnsureDir creates the directory (and parents) if it does not exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

