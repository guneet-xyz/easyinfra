package k3s

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/spf13/cobra"
)

type uninstallFlags struct {
	all bool
	yes bool
}

func newUninstallCmd(rootF *RootFlags) *cobra.Command {
	f := &uninstallFlags{}
	cmd := &cobra.Command{
		Use:   "uninstall [app]",
		Short: "Uninstall one or all apps via helm uninstall",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(cmd, rootF, args, f)
		},
	}
	cmd.Flags().BoolVar(&f.all, "all", false, "uninstall all apps in reverse order")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "skip confirmation prompt for --all")
	return cmd
}

func runUninstall(cmd *cobra.Command, flags *RootFlags, args []string, f *uninstallFlags) error {
	op, err := prepareOp(cmd, flags, args, f.all)
	if err != nil {
		return err
	}
	apps := op.apps
	if f.all {
		if !f.yes && !confirmAll(cmd) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Uninstall cancelled")
			return nil
		}
		apps = reverseApps(apps)
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	out := cmd.OutOrStdout()
	for _, app := range apps {
		if err := op.client.Uninstall(ctx, app.Name, app.Namespace); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "Uninstalled %s\n", app.Name)
	}
	return nil
}

func confirmAll(cmd *cobra.Command) bool {
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Uninstall all apps? [y/N] ")
	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		return false
	}
	resp := strings.TrimSpace(scanner.Text())
	return resp == "y" || resp == "Y"
}

func reverseApps(in []config.AppConfig) []config.AppConfig {
	out := make([]config.AppConfig, len(in))
	for i, a := range in {
		out[len(in)-1-i] = a
	}
	return out
}
