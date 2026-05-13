package phases

import (
	"strings"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/config"
)

func names(phase []config.AppConfigV2) []string {
	out := make([]string, len(phase))
	for i, a := range phase {
		out[i] = a.Name
	}
	return out
}

func TestResolve_NoPhases_AllAppsInDefault(t *testing.T) {
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{
			{Name: "caddy"},
			{Name: "walls", DependsOn: []string{"caddy"}},
			{Name: "db"},
		},
	}

	phases, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}

	got := names(phases[0])
	// topo.Sort breaks ties alphabetically: caddy, db, then walls (depends on caddy).
	want := []string{"caddy", "db", "walls"}
	if !equal(got, want) {
		t.Fatalf("default phase apps = %v, want %v", got, want)
	}
}

func TestResolve_SinglePhase_RestInDefault(t *testing.T) {
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{
			{Name: "cert-manager"},
			{Name: "trust-manager", DependsOn: []string{"cert-manager"}},
			{Name: "caddy"},
			{Name: "walls"},
		},
		Phases: []config.PhaseConfig{
			{Name: "bootstrap", Apps: []string{"cert-manager", "trust-manager"}},
		},
	}

	phases, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases (bootstrap + default), got %d", len(phases))
	}

	got0 := names(phases[0])
	want0 := []string{"cert-manager", "trust-manager"}
	if !equal(got0, want0) {
		t.Fatalf("bootstrap phase = %v, want %v", got0, want0)
	}

	got1 := names(phases[1])
	want1 := []string{"caddy", "walls"}
	if !equal(got1, want1) {
		t.Fatalf("default phase = %v, want %v", got1, want1)
	}
}

func TestResolve_AppInTwoPhases_Error(t *testing.T) {
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{
			{Name: "caddy"},
			{Name: "walls"},
		},
		Phases: []config.PhaseConfig{
			{Name: "first", Apps: []string{"caddy"}},
			{Name: "second", Apps: []string{"caddy", "walls"}},
		},
	}

	_, err := Resolve(cfg)
	if err == nil {
		t.Fatal("expected error for app in two phases, got nil")
	}
	if !strings.Contains(err.Error(), "caddy") {
		t.Fatalf("error should mention duplicated app: %v", err)
	}
}

func TestResolve_AppInNoPhase_GoesToDefault(t *testing.T) {
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{
			{Name: "caddy"},
			{Name: "walls"},
			{Name: "db"},
		},
		Phases: []config.PhaseConfig{
			{Name: "bootstrap", Apps: []string{"caddy"}},
		},
	}

	phases, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(phases))
	}

	got := names(phases[1])
	want := []string{"db", "walls"}
	if !equal(got, want) {
		t.Fatalf("default phase = %v, want %v", got, want)
	}
}

func TestResolve_UnknownAppInPhase_Error(t *testing.T) {
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{
			{Name: "caddy"},
		},
		Phases: []config.PhaseConfig{
			{Name: "bootstrap", Apps: []string{"ghost"}},
		},
	}

	_, err := Resolve(cfg)
	if err == nil {
		t.Fatal("expected error for unknown app reference, got nil")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("error should mention unknown app: %v", err)
	}
}

func TestResolve_NoApps_NoPhases(t *testing.T) {
	cfg := &config.InfraConfigV2{}
	phases, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(phases) != 0 {
		t.Fatalf("expected 0 phases for empty config, got %d", len(phases))
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
