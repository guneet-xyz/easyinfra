package config

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestExampleParses reads example_infra.yaml, unmarshals it, and verifies structure.
func TestExampleParses(t *testing.T) {
	data, err := os.ReadFile("example_infra.yaml")
	if err != nil {
		t.Fatalf("failed to read example_infra.yaml: %v", err)
	}

	var cfg InfraConfig
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		t.Fatalf("failed to unmarshal example_infra.yaml: %v", err)
	}

	if cfg.KubeContext != "pax" {
		t.Errorf("expected kubeContext=pax, got %q", cfg.KubeContext)
	}

	if len(cfg.Apps) != 11 {
		t.Errorf("expected 11 apps, got %d", len(cfg.Apps))
	}

	t.Logf("✓ 11 apps loaded from example_infra.yaml")
}

// TestRoundTrip unmarshals, marshals, then unmarshals again to verify no field loss.
func TestRoundTrip(t *testing.T) {
	data, err := os.ReadFile("example_infra.yaml")
	if err != nil {
		t.Fatalf("failed to read example_infra.yaml: %v", err)
	}

	// First unmarshal
	var cfg1 InfraConfig
	err = yaml.Unmarshal(data, &cfg1)
	if err != nil {
		t.Fatalf("first unmarshal failed: %v", err)
	}

	// Marshal back to YAML
	marshaled, err := yaml.Marshal(&cfg1)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Second unmarshal
	var cfg2 InfraConfig
	err = yaml.Unmarshal(marshaled, &cfg2)
	if err != nil {
		t.Fatalf("second unmarshal failed: %v", err)
	}

	// Verify key fields match
	if cfg1.KubeContext != cfg2.KubeContext {
		t.Errorf("kubeContext mismatch: %q vs %q", cfg1.KubeContext, cfg2.KubeContext)
	}

	if len(cfg1.Apps) != len(cfg2.Apps) {
		t.Errorf("app count mismatch: %d vs %d", len(cfg1.Apps), len(cfg2.Apps))
	}

	for i, app1 := range cfg1.Apps {
		app2 := cfg2.Apps[i]
		if app1.Name != app2.Name {
			t.Errorf("app[%d] name mismatch: %q vs %q", i, app1.Name, app2.Name)
		}
		if app1.Chart != app2.Chart {
			t.Errorf("app[%d] chart mismatch: %q vs %q", i, app1.Chart, app2.Chart)
		}
		if app1.Namespace != app2.Namespace {
			t.Errorf("app[%d] namespace mismatch: %q vs %q", i, app1.Namespace, app2.Namespace)
		}
		if app1.Order != app2.Order {
			t.Errorf("app[%d] order mismatch: %d vs %d", i, app1.Order, app2.Order)
		}
	}

	t.Logf("✓ Round-trip successful, no field loss")
}
