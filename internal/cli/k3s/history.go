package k3s

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/history"
	"github.com/spf13/cobra"
)

func newHistoryCmd(flags *RootFlags) *cobra.Command {
	var (
		asJSON    bool
		max       int
		namespace string
	)
	cmd := &cobra.Command{
		Use:   "history <app>",
		Short: "Show helm release history for an app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistory(cmd, flags, args[0], namespace, max, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON output")
	cmd.Flags().IntVar(&max, "max", 10, "maximum number of revisions to return")
	cmd.Flags().StringVar(&namespace, "namespace", "", "override namespace from config")
	return cmd
}

func runHistory(cmd *cobra.Command, flags *RootFlags, appName, nsOverride string, max int, asJSON bool) error {
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
	revs, err := history.History(cmd.Context(), runner, app.Name, ns, max)
	if err != nil {
		return err
	}

	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(revs)
	}

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
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
	return tw.Flush()
}

func findApp(cfg *config.InfraConfig, name string) (*config.AppConfig, error) {
	for i := range cfg.Apps {
		if cfg.Apps[i].Name == name {
			return &cfg.Apps[i], nil
		}
	}
	return nil, fmt.Errorf("unknown app %q", name)
}
