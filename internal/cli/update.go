package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newUpdateCmd(rootF *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Pull the latest changes from the infrastructure repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			mgr, err := newRepoManager(cmd, rootF)
			if err != nil {
				return err
			}
			if !mgr.Exists() {
				return fmt.Errorf("no infra repo at %s; run `easyinfra init <url>` first", mgr.RepoDir)
			}
			if err := mgr.Pull(cmd.Context()); err != nil {
				return err
			}
		status, err := mgr.Status(cmd.Context())
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated %s (HEAD: %s)\n", mgr.RepoDir, status.Commit)
		return nil
		},
	}
}
