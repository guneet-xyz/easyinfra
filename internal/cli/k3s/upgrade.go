package k3s

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type upgradeFlags struct {
	all bool
}

func newUpgradeCmd(rootF *RootFlags) *cobra.Command {
	f := &upgradeFlags{}
	cmd := &cobra.Command{
		Use:   "upgrade [app]",
		Short: "Upgrade one or all apps via helm upgrade",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runK3sUpgrade(cmd, rootF, args, f.all)
		},
	}
	cmd.Flags().BoolVar(&f.all, "all", false, "upgrade all apps in order")
	return cmd
}

func runK3sUpgrade(cmd *cobra.Command, flags *RootFlags, args []string, all bool) error {
	op, err := prepareOp(cmd, flags, args, all)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	out := cmd.OutOrStdout()
	for _, app := range op.apps {
		opts := buildInstallOpts(op.cfg, op.baseDir, app)
		if err := op.client.Upgrade(ctx, opts); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "Upgraded %s\n", app.Name)
	}
	return nil
}
