package cmd

import (
	"github.com/spf13/cobra"
)

var utilsCmd = &cobra.Command{
	Use:   "utils",
	Short: "Utility commands",
	Long:  `Utility commands for configuration, debugging, and maintenance.`,
}

func init() {
	rootCmd.AddCommand(utilsCmd)
}

