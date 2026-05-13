package k3s

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/guneet-xyz/easyinfra/pkg/discover"
	"github.com/guneet-xyz/easyinfra/pkg/migrate"
	"github.com/spf13/cobra"
)

func newMigrateCmd(flags *RootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate from legacy shell scripts to easyinfra",
	}
	cmd.AddCommand(
		newMigrateExplainCmd(flags),
		newMigrateGenerateConfigCmd(flags),
	)
	return cmd
}

func newMigrateExplainCmd(_ *RootFlags) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Print a mapping table of legacy scripts to easyinfra commands",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format := "markdown"
			if asJSON {
				format = "json"
			}
			return migrate.Render(cmd.OutOrStdout(), format)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the mapping table as JSON")
	return cmd
}

func newMigrateGenerateConfigCmd(_ *RootFlags) *cobra.Command {
	var writeInPlace bool
	cmd := &cobra.Command{
		Use:   "generate-config <path>",
		Short: "Scan a legacy infra directory and emit a v2 infra.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			layout, err := discover.Scan(path)
			if err != nil {
				return err
			}
			// Override Root so cluster.name (filepath.Base of layout.Root) reflects
			// the absolute directory name even when path was given as "." or relative.
			layout.Root = absPath

			var buf bytes.Buffer
			if err := discover.Emit(layout, &buf); err != nil {
				return fmt.Errorf("emit infra.yaml: %w", err)
			}

			if writeInPlace {
				dest := filepath.Join(path, "infra.yaml")
				if err := os.WriteFile(dest, buf.Bytes(), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", dest, err)
				}
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", dest)
				return nil
			}
			_, err = cmd.OutOrStdout().Write(buf.Bytes())
			return err
		},
	}
	cmd.Flags().BoolVar(&writeInPlace, "write", false, "write infra.yaml into <path>/infra.yaml")
	return cmd
}
