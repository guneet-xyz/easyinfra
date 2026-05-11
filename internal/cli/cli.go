// Package cli provides the command-line interface for easyinfra.
package cli

import "os"

// Execute initializes and runs the root cobra command.
func Execute(version, commit, date string) {
	rootCmd := newRootCmd(version, commit, date)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
