package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  `Print the version number of the Cora CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cora version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
