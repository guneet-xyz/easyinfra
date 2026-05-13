package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestBinaryCheckFound(t *testing.T) {
	c := BinaryCheck{Binary: "go", InstallURL: "https://go.dev"}
	res := c.Run(context.Background())
	assert.Equal(t, StatusOK, res.Status)
	assert.Contains(t, res.Message, "go found")
}

func TestBinaryCheckMissing(t *testing.T) {
	c := BinaryCheck{Binary: "definitely-not-a-real-binary-xyz", InstallURL: "https://example.com"}
	res := c.Run(context.Background())
	assert.Equal(t, StatusFail, res.Status)
	assert.Contains(t, res.Message, "not found")
	assert.Contains(t, res.Detail, "https://example.com")
}

func TestKubeContextCheckNoCluster(t *testing.T) {
	c := KubeContextCheck{Context: "pax", NoCluster: true}
	res := c.Run(context.Background())
	assert.Equal(t, StatusWarn, res.Status)
	assert.Contains(t, res.Message, "skipped")
}

func TestChartPathsCheckMissing(t *testing.T) {
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{
			{Name: "caddy", Chart: "apps/caddy"},
		},
	}
	c := ChartPathsCheck{Cfg: cfg, BaseDir: "/nonexistent"}
	res := c.Run(context.Background())
	assert.Equal(t, StatusFail, res.Status)
	assert.Contains(t, res.Detail, "apps/caddy")
}

func TestChartPathsCheckOK(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "apps", "caddy")
	_ = os.MkdirAll(chartDir, 0755)
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{
			{Name: "caddy", Chart: "apps/caddy"},
		},
	}
	c := ChartPathsCheck{Cfg: cfg, BaseDir: dir}
	res := c.Run(context.Background())
	assert.Equal(t, StatusOK, res.Status)
}

func TestBackupConfigCheckNoPVCs(t *testing.T) {
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{{Name: "headlamp", Chart: "apps/headlamp"}},
	}
	c := BackupConfigCheck{Cfg: cfg}
	res := c.Run(context.Background())
	assert.Equal(t, StatusOK, res.Status)
}

func TestBackupConfigCheckMissingHost(t *testing.T) {
	cfg := &config.InfraConfigV2{
		Apps: []config.AppConfigV2{{Name: "caddy", Chart: "apps/caddy", PVCs: []string{"caddy-data"}}},
	}
	c := BackupConfigCheck{Cfg: cfg}
	res := c.Run(context.Background())
	assert.Equal(t, StatusFail, res.Status)
	assert.Contains(t, res.Message, "remoteHost")
}

func TestRunReport(t *testing.T) {
	checks := []Check{
		BinaryCheck{Binary: "go", InstallURL: "https://go.dev"},
		BinaryCheck{Binary: "definitely-not-real-xyz", InstallURL: "https://example.com"},
	}
	report := Run(context.Background(), checks)
	assert.Len(t, report.Checks, 2)
	assert.True(t, report.Failed)
	assert.Equal(t, StatusOK, report.Checks[0].Status)
	assert.Equal(t, StatusFail, report.Checks[1].Status)
}
