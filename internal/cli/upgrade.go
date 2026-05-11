package cli

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/guneet-xyz/easyinfra/pkg/selfupdate"
	"github.com/spf13/cobra"
)

type updaterInterface interface {
	Check(ctx context.Context) (*selfupdate.CheckResult, error)
	Update(ctx context.Context) (string, error)
}

var newUpdater = func(version string) updaterInterface {
	return &selfupdate.Updater{
		Owner:          "guneet-xyz",
		Repo:           "easyinfra",
		CurrentVersion: version,
	}
}

type upgradeFlags struct {
	check bool
	yes   bool
}

func newUpgradeCmd(version string) *cobra.Command {
	f := &upgradeFlags{}
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade easyinfra to the latest version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUpgrade(cmd, version, f)
		},
	}
	cmd.Flags().BoolVar(&f.check, "check", false, "only check for updates, don't apply")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "skip confirmation prompt")
	return cmd
}

func runUpgrade(cmd *cobra.Command, version string, flags *upgradeFlags) error {
	ctx := cmd.Context()
	updater := newUpdater(version)

	// Check for updates
	result, err := updater.Check(ctx)
	if err != nil {
		return err
	}

	// If no update available
	if !result.HasUpdate {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Already on latest (%s)\n", version)
		return nil
	}

	// If --check flag, just report and exit
	if flags.check {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s → %s\n", version, result.LatestVersion)
		return nil
	}

	// Prompt for confirmation unless --yes
	if !flags.yes {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Update easyinfra %s → %s? [y/N] ", version, result.LatestVersion)
		scanner := bufio.NewScanner(cmd.InOrStdin())
		if !scanner.Scan() {
			return scanner.Err()
		}
		response := strings.TrimSpace(scanner.Text())
		if response != "y" && response != "Y" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Upgrade cancelled\n")
			return nil
		}
	}

	// Apply update
	newVersion, err := updater.Update(ctx)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Upgraded to %s\n", newVersion)
	return nil
}
