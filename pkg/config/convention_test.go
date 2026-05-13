package config

import (
	"testing"
)

func TestAppLocalDiscovery(t *testing.T) {
	cfg, err := LoadV2("testdata/convention/infra.yaml")
	if err != nil {
		t.Fatalf("LoadV2 failed: %v", err)
	}
	if len(cfg.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(cfg.Apps))
	}
	app := cfg.Apps[0]
	if app.Name != "walls" {
		t.Errorf("expected app name %q, got %q", "walls", app.Name)
	}
	if app.Chart != "apps/walls" {
		t.Errorf("expected chart %q, got %q", "apps/walls", app.Chart)
	}
	if len(app.ValueFiles) != 1 || app.ValueFiles[0] != "apps/walls/values.yaml" {
		t.Errorf("expected valueFiles [apps/walls/values.yaml], got %v", app.ValueFiles)
	}
	if app.Namespace != "walls" {
		t.Errorf("expected namespace defaulted to %q, got %q", "walls", app.Namespace)
	}
}
