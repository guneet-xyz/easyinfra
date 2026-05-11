package k3s

import (
	"github.com/guneet/easyinfra/pkg/exec"
	"github.com/spf13/cobra"
)

// RootFlags contains global flags for k3s commands.
type RootFlags struct {
	Config         string
	DryRun         bool
	Verbose        bool
	ConfirmContext bool
}

var newRunner = func(cmd *cobra.Command, flags *RootFlags) exec.Runner {
	return &exec.RealRunner{
		DryRun:  flags.DryRun,
		Verbose: flags.Verbose,
		Stdout:  cmd.OutOrStdout(),
		Stderr:  cmd.ErrOrStderr(),
	}
}

// NewK3sCmd creates the k3s command with all its subcommands.
func NewK3sCmd(flags *RootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k3s",
		Short: "Manage k3s applications via Helm",
	}
	cmd.AddCommand(
		newValidateCmd(flags),
		newBackupCmd(flags),
		newRestoreCmd(flags),
		newInstallCmd(flags),
		newUpgradeCmd(flags),
		newUninstallCmd(flags),
	)
	return cmd
}
