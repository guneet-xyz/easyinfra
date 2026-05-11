package cli

import "os"

func Execute(version, commit, date string) {
	rootCmd := newRootCmd(version, commit, date)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
