package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print build information",
	Run: func(*cobra.Command, []string) {
		fmt.Printf("version:    %s\ncommit:     %s\nbuilt:      %s\n",
			version, commit, buildTime)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
