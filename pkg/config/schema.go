package config

import "gopkg.in/yaml.v3"

const (
	APIVersionV1 = ""
	APIVersionV2 = "easyinfra/v2"
)

// DetectAPIVersion reads the apiVersion field from raw YAML bytes.
func DetectAPIVersion(data []byte) string {
	var probe struct {
		APIVersion string `yaml:"apiVersion"`
	}
	_ = yaml.Unmarshal(data, &probe)
	if probe.APIVersion == APIVersionV2 {
		return APIVersionV2
	}
	return APIVersionV1
}
