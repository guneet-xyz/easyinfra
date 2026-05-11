package cli

import (
	"fmt"

	"github.com/guneet/easyinfra/pkg/exec"
	"github.com/guneet/easyinfra/pkg/paths"
	"github.com/guneet/easyinfra/pkg/repo"
	"github.com/spf13/cobra"
)

var newRepoManager = func(cmd *cobra.Command, flags *rootFlags) (*repo.Manager, error) {
	repoDir, err := paths.RepoDir()
	if err != nil {
		return nil, fmt.Errorf("resolving repo dir: %w", err)
	}
	runner := &exec.RealRunner{
		DryRun:  flags.dryRun,
		Verbose: flags.verbose,
		Stdout:  cmd.OutOrStdout(),
		Stderr:  cmd.ErrOrStderr(),
	}
	return &repo.Manager{Runner: runner, RepoDir: repoDir}, nil
}

type initFlags struct {
	branch string
	force  bool
}

func newInitCmd(rootF *rootFlags) *cobra.Command {
	f := &initFlags{}
	cmd := &cobra.Command{
		Use:   "init <git-url>",
		Short: "Clone an infrastructure repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := newRepoManager(cmd, rootF)
			if err != nil {
				return err
			}
		if err := mgr.Clone(cmd.Context(), args[0], f.branch, f.force); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Cloned %s to %s\n", args[0], mgr.RepoDir)
		return nil
		},
	}
	cmd.Flags().StringVar(&f.branch, "branch", "", "branch to clone")
	cmd.Flags().BoolVar(&f.force, "force", false, "overwrite existing repo")
	return cmd
}
