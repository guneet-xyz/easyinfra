package config

// InfraConfigV2 is the top-level structure for infra.yaml v2.
type InfraConfigV2 struct {
	APIVersion string          `yaml:"apiVersion"`
	Cluster    ClusterConfig   `yaml:"cluster"`
	Defaults   Defaults        `yaml:"defaults"`
	Rendering  RenderingConfig `yaml:"rendering,omitempty"`
	Restore    RestoreConfig   `yaml:"restore,omitempty"`
	Backup     BackupConfigV2  `yaml:"backup"`
	Phases     []PhaseConfig   `yaml:"phases,omitempty"`
	Includes   []string        `yaml:"includes,omitempty"`
	Apps       []AppConfigV2   `yaml:"apps"`
}

// ClusterConfig describes the target cluster.
type ClusterConfig struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	KubeContext string `yaml:"kubeContext"`
	Kubeconfig  string `yaml:"kubeconfig,omitempty"`
}

// RenderingConfig controls helm template/render behavior.
type RenderingConfig struct {
	AllowMissingCluster bool   `yaml:"allowMissingCluster"`
	PostRendererInCI    string `yaml:"postRendererInCI"`
	OutputDir           string `yaml:"outputDir,omitempty"`
}

// RestoreConfig controls restore behavior for PVCs.
type RestoreConfig struct {
	Overwrite string `yaml:"overwrite"`
}

// PhaseConfig groups apps into ordered install phases.
type PhaseConfig struct {
	Name string   `yaml:"name"`
	Apps []string `yaml:"apps"`
}

// BackupConfigV2 is the v2 backup configuration with retention.
type BackupConfigV2 struct {
	RemoteHost string          `yaml:"remoteHost"`
	RemoteUser string          `yaml:"remoteUser,omitempty"`
	RemoteTmp  string          `yaml:"remoteTmp"`
	LocalDir   string          `yaml:"localDir"`
	Retention  RetentionConfig `yaml:"retention,omitempty"`
}

// RetentionConfig controls how long backups are kept.
type RetentionConfig struct {
	Keep      int    `yaml:"keep,omitempty"`
	OlderThan string `yaml:"olderThan,omitempty"`
}

// AppLocal is the schema for an app-local easyinfra.yaml file used by
// convention-based discovery (apps/<name>/easyinfra.yaml).
//
// Paths in ValueFiles are interpreted as relative to the app directory
// containing the easyinfra.yaml file. The chart path is interpreted as
// relative to the parent infra.yaml directory.
type AppLocal struct {
	Name         string        `yaml:"name"`
	Chart        string        `yaml:"chart"`
	DependsOn    []string      `yaml:"dependsOn,omitempty"`
	PVCs         []string      `yaml:"pvcs,omitempty"`
	ValueFiles   []string      `yaml:"valueFiles,omitempty"`
	PostRenderer *PostRenderer `yaml:"postRenderer,omitempty"`
}

// AppConfigV2 describes an application in v2.
type AppConfigV2 struct {
	Name         string        `yaml:"name"`
	Chart        string        `yaml:"chart"`
	Namespace    string        `yaml:"namespace,omitempty"`
	Order        int           `yaml:"order,omitempty"`
	ValueFiles   []string      `yaml:"valueFiles,omitempty"`
	PostRenderer *PostRenderer `yaml:"postRenderer,omitempty"`
	DependsOn    []string      `yaml:"dependsOn,omitempty"`
	PVCs         []string      `yaml:"pvcs,omitempty"`
}
