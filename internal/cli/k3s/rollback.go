package k3s

import (
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/guneet-xyz/easyinfra/pkg/helm"
	"github.com/guneet-xyz/easyinfra/pkg/history"
	"github.com/spf13/cobra"
)

func newRollbackCmd(flags *RootFlags) *cobra.Command {
	var (
		force     bool
		wait      bool
		namespace string
	)
	cmd := &cobra.Command{
		Use:   "rollback <app> [revision]",
		Short: "Rollback a helm release to a previous revision",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			revision := 0
			if len(args) == 2 {
				r, err := strconv.Atoi(args[1])
				if err != nil {
					return fmt.Errorf("invalid revision %q: %w", args[1], err)
				}
				if r <= 0 {
					return fmt.Errorf("revision must be positive, got %d", r)
				}
				revision = r
			}
			return runRollback(cmd, flags, args[0], namespace, revision, force, wait)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "force rollback (required when revision is omitted)")
	cmd.Flags().BoolVar(&wait, "wait", true, "wait for resources to be ready")
	cmd.Flags().StringVar(&namespace, "namespace", "", "override namespace from config")
	return cmd
}

func runRollback(cmd *cobra.Command, flags *RootFlags, appName, nsOverride string, revision int, force, wait bool) error {
	cfg, _, err := loadConfig(flags)
	if err != nil {
		return err
	}
	app, err := findApp(cfg, appName)
	if err != nil {
		return err
	}
	ns := app.Namespace
	if nsOverride != "" {
		ns = nsOverride
	}
	if ns == "" {
		return fmt.Errorf("app %q has no namespace; pass --namespace", appName)
	}

	runner := newRunner(cmd, flags)

	if revision == 0 && !force {
		revs, herr := history.History(cmd.Context(), runner, app.Name, ns, 10)
		if herr != nil {
			return herr
		}
		tw := tabwriter.NewWriter(cmd.ErrOrStderr(), 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "REV\tUPDATED\tSTATUS\tCHART\tDESCRIPTION"); err != nil {
			return err
		}
		for _, r := range revs {
			if _, err := fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
				r.Revision,
				r.Updated.Format("2006-01-02 15:04:05"),
				r.Status,
				r.Chart,
				r.Description,
			); err != nil {
				return err
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		return fmt.Errorf("specify revision or pass --force")
	}

	client := &helm.Client{Runner: runner}
	return client.Rollback(cmd.Context(), app.Name, ns, revision, force, wait)
}
