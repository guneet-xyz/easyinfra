package k3s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/guneet-xyz/easyinfra/pkg/config"
	"github.com/guneet-xyz/easyinfra/pkg/exec"
	"github.com/guneet-xyz/easyinfra/pkg/status"
	"github.com/spf13/cobra"
)

type statusEntry struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Chart     string    `json:"chart"`
	Revision  int       `json:"revision"`
	Status    string    `json:"status"`
	Updated   time.Time `json:"updated"`
}

func newStatusCmd(flags *RootFlags) *cobra.Command {
	var (
		all    bool
		asJSON bool
		watch  bool
	)
	cmd := &cobra.Command{
		Use:   "status [app]",
		Short: "Show helm release status for one or all apps",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, flags, args, all, asJSON, watch)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "show status for all apps")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON output")
	cmd.Flags().BoolVar(&watch, "watch", false, "re-poll every 5s (ctrl-c to exit)")
	return cmd
}

func runStatus(cmd *cobra.Command, flags *RootFlags, args []string, all, asJSON, watch bool) error {
	cfg, _, err := loadConfig(flags)
	if err != nil {
		return err
	}
	apps, err := selectOpApps(cfg, args, all)
	if err != nil {
		return err
	}
	runner := newRunner(cmd, flags)
	out := cmd.OutOrStdout()

	if !watch {
		entries, err := collectStatuses(cmd.Context(), runner, apps)
		if err != nil {
			return err
		}
		return renderStatuses(out, entries, asJSON)
	}

	for {
		if _, err := fmt.Fprint(out, "\033[2J\033[H"); err != nil {
			return err
		}
		entries, err := collectStatuses(cmd.Context(), runner, apps)
		if err != nil {
			return err
		}
		if err := renderStatuses(out, entries, asJSON); err != nil {
			return err
		}
		select {
		case <-cmd.Context().Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

func collectStatuses(ctx context.Context, runner exec.Runner, apps []config.AppConfig) ([]statusEntry, error) {
	entries := make([]statusEntry, 0, len(apps))
	for _, app := range apps {
		ns := app.Namespace
		rel, err := status.Status(ctx, runner, app.Name, ns)
		if err != nil {
			if errors.Is(err, status.ErrNotFound) {
				entries = append(entries, statusEntry{
					Name:      app.Name,
					Namespace: ns,
					Status:    "not-deployed",
				})
				continue
			}
			return nil, err
		}
		chart := rel.Chart.Name
		if rel.Chart.Version != "" {
			chart = fmt.Sprintf("%s-%s", rel.Chart.Name, rel.Chart.Version)
		}
		entries = append(entries, statusEntry{
			Name:      rel.Name,
			Namespace: rel.Namespace,
			Chart:     chart,
			Revision:  rel.Revision,
			Status:    rel.Status,
			Updated:   rel.Updated,
		})
	}
	return entries, nil
}

func renderStatuses(w io.Writer, entries []statusEntry, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tNS\tCHART\tREV\tSTATUS\tUPDATED"); err != nil {
		return err
	}
	for _, e := range entries {
		updated := ""
		if !e.Updated.IsZero() {
			updated = e.Updated.Format("2006-01-02 15:04:05")
		}
		rev := ""
		if e.Revision > 0 {
			rev = fmt.Sprintf("%d", e.Revision)
		}
		chart := e.Chart
		if chart == "" {
			chart = "-"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			e.Name, e.Namespace, chart, rev, e.Status, updated,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}
