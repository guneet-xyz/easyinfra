package k3s

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/guneet-xyz/easyinfra/pkg/discover"
	"github.com/spf13/cobra"
)

func newDiscoverCmd(flags *RootFlags) *cobra.Command {
	var (
		writeInPlace bool
		outFile      string
	)
	cmd := &cobra.Command{
		Use:   "discover <path>",
		Short: "Scan an infra directory and emit a v2 infra.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiscover(cmd, flags, args[0], writeInPlace, outFile)
		},
	}
	cmd.Flags().BoolVar(&writeInPlace, "write", false, "write infra.yaml into <path>/infra.yaml")
	cmd.Flags().StringVarP(&outFile, "output", "o", "", "write output to the given file")
	return cmd
}

func runDiscover(cmd *cobra.Command, _ *RootFlags, path string, writeInPlace bool, outFile string) error {
	layout, err := discover.Scan(path)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := discover.Emit(layout, &buf); err != nil {
		return fmt.Errorf("emit infra.yaml: %w", err)
	}

	switch {
	case writeInPlace && outFile != "":
		return fmt.Errorf("--write and -o are mutually exclusive")
	case writeInPlace:
		dest := filepath.Join(path, "infra.yaml")
		if err := os.WriteFile(dest, buf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", dest)
	case outFile != "":
		if err := os.WriteFile(outFile, buf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outFile, err)
		}
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", outFile)
	default:
		if _, err := cmd.OutOrStdout().Write(buf.Bytes()); err != nil {
			return err
		}
	}
	return nil
}
