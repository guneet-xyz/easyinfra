package config

// InfraConfig is the top-level structure for infra.yaml.
type InfraConfig struct {
	KubeContext string       `yaml:"kubeContext"`
	Defaults    Defaults     `yaml:"defaults"`
	Backup      BackupConfig `yaml:"backup"`
	Apps        []AppConfig  `yaml:"apps"`
}

// Defaults holds values applied to all apps unless overridden.
type Defaults struct {
	PostRenderer *PostRenderer `yaml:"postRenderer,omitempty"`
	ValueFiles   []string      `yaml:"valueFiles,omitempty"`
}

// PostRenderer configures a helm post-renderer.
type PostRenderer struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

// BackupConfig holds SSH/SCP backup configuration.
type BackupConfig struct {
	RemoteHost string `yaml:"remoteHost"`
	RemoteUser string `yaml:"remoteUser,omitempty"`
	RemoteTmp  string `yaml:"remoteTmp"`
	LocalDir   string `yaml:"localDir"`
}

// AppConfig describes a single application/helm chart.
type AppConfig struct {
	Name         string        `yaml:"name"`
	Chart        string        `yaml:"chart"`
	Namespace    string        `yaml:"namespace"`
	Order        int           `yaml:"order"`
	ValueFiles   []string      `yaml:"valueFiles,omitempty"`
	PostRenderer *PostRenderer `yaml:"postRenderer,omitempty"`
	DependsOn    []string      `yaml:"dependsOn,omitempty"`
	PVCs         []string      `yaml:"pvcs,omitempty"`
}
