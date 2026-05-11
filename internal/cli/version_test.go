package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd("1.2.3", "deadbeef", "2026-05-11")
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"version"})
	err := cmd.Execute()
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "easyinfra 1.2.3")
	require.Contains(t, out, "deadbeef")
	require.Contains(t, out, "2026-05-11")
}

func TestRootHelp(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd("dev", "unknown", "unknown")
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})
	_ = cmd.Execute()
	out := buf.String()
	require.True(t, strings.Contains(out, "--config") || strings.Contains(out, "config"), "expected --config flag in help")
	require.True(t, strings.Contains(out, "--dry-run") || strings.Contains(out, "dry-run"), "expected --dry-run flag in help")
}
