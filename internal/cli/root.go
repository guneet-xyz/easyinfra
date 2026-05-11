package cli

import (
	"github.com/guneet/easyinfra/internal/cli/k3s"
	"github.com/spf13/cobra"
)

type rootFlags struct {
	config         string
	dryRun         bool
	verbose        bool
	confirmContext bool
}

func newRootCmd(version, commit, date string) *cobra.Command {
	flags := &rootFlags{}

	cmd := &cobra.Command{
		Use:   "easyinfra",
		Short: "Manage k3s infrastructure via config-as-code",
		Long: `easyinfra is a config-driven CLI for managing k3s infrastructure.

It replaces deploy.sh, validate.sh, and backup.sh with a single tool
driven by infra.yaml in your infrastructure repository.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&flags.config, "config", "", "path to infra.yaml (default: ~/.config/easyinfra/repo/infra.yaml)")
	cmd.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "print commands without executing them")
	cmd.PersistentFlags().BoolVar(&flags.verbose, "verbose", false, "enable verbose output")
	cmd.PersistentFlags().BoolVar(&flags.confirmContext, "confirm-context", false, "proceed even if kubeContext in infra.yaml does not match current context")

	cmd.AddCommand(newVersionCmd(version, commit, date))
	cmd.AddCommand(newInitCmd(flags), newUpdateCmd(flags))
	cmd.AddCommand(newUpgradeCmd(version))

	k3sFlags := &k3s.RootFlags{}
	k3sCmd := k3s.NewK3sCmd(k3sFlags)
	k3sCmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		k3sFlags.Config = flags.config
		k3sFlags.DryRun = flags.dryRun
		k3sFlags.Verbose = flags.verbose
		k3sFlags.ConfirmContext = flags.confirmContext
		return nil
	}
	cmd.AddCommand(k3sCmd)

	return cmd
}
