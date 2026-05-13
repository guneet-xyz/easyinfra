package cli

import (
	"fmt"
	"path/filepath"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/doctor"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/guneet-xyz/easyinfra/pkg/output"
	"github.com/guneet-xyz/easyinfra/pkg/paths"
	"github.com/spf13/cobra"
)

type doctorFlags struct {
	noCluster bool
	jsonOut   bool
}

func newDoctorCmd(rootF *rootFlags) *cobra.Command {
	f := &doctorFlags{}
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run preflight checks against the configured infrastructure",
		Long: `doctor runs a series of preflight checks: required binaries on PATH,
infra.yaml validity, chart paths existence, backup config, and (unless
--no-cluster is set) kube-context reachability.

Exits 0 when all checks pass or only emit warnings; exits 1 if any
check reports FAIL.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfgPath, err := resolveConfigPath(rootF.config)
			if err != nil {
				return err
			}

			cfg, err := config.LoadV2(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config %s: %w", cfgPath, err)
			}

			runner := &exec.RealRunner{
				DryRun:  rootF.dryRun,
				Verbose: rootF.verbose,
				Stdout:  cmd.OutOrStdout(),
				Stderr:  cmd.ErrOrStderr(),
			}

			baseDir := filepath.Dir(cfgPath)
			checks := doctor.DefaultChecks(cfg, baseDir, runner, f.noCluster)
			report := doctor.Run(cmd.Context(), checks)

			var writer output.Writer = output.TextWriter{}
			if f.jsonOut {
				writer = output.JSONWriter{}
			}
			if err := writer.WriteReport(cmd.OutOrStdout(), report); err != nil {
				return err
			}

			if report.Failed {
				return fmt.Errorf("doctor: one or more checks failed")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&f.noCluster, "no-cluster", false, "skip checks that require a reachable cluster")
	cmd.Flags().BoolVar(&f.jsonOut, "json", false, "emit results as JSON")
	return cmd
}

func resolveConfigPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	p, err := paths.DefaultConfigPath()
	if err != nil {
		return "", fmt.Errorf("resolving default config path: %w", err)
	}
	return p, nil
}
