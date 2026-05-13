package postrender

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/config"
)

func TestValidateMissingBinary(t *testing.T) {
	t.Setenv("PATH", "/dev/null")
	err := Validate(&config.PostRenderer{Command: "definitely-not-a-real-binary-xyz"})
	if !errors.Is(err, ErrBinaryMissing) {
		t.Fatalf("want ErrBinaryMissing, got %v", err)
	}
}

func TestProbeMissingBinary(t *testing.T) {
	t.Setenv("PATH", "/dev/null")
	res := Probe(&config.PostRenderer{Command: "definitely-not-a-real-binary-xyz"})
	if res.Found {
		t.Fatalf("want Found=false, got %+v", res)
	}
}

func TestValidateRealBinary(t *testing.T) {
	bin := findBinary(t)
	if err := Validate(&config.PostRenderer{Command: bin}); err != nil {
		t.Fatalf("Validate(%s) = %v, want nil", bin, err)
	}
}

func TestProbeRealBinary(t *testing.T) {
	bin := findBinary(t)
	res := Probe(&config.PostRenderer{Command: bin})
	if !res.Found {
		t.Fatalf("want Found=true, got %+v", res)
	}
	if res.Path == "" {
		t.Fatalf("want non-empty Path, got %+v", res)
	}
}

func TestValidateNilConfig(t *testing.T) {
	if err := Validate(nil); !errors.Is(err, ErrBinaryMissing) {
		t.Fatalf("want ErrBinaryMissing, got %v", err)
	}
}

func findBinary(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"echo", "true", "sh"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	t.Skip("no probe-friendly binary available on PATH")
	return ""
}
