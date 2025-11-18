package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jira",
	Short: "A CLI tool to streamline Jira workflows",
	Long: `go-jira-helper is a command-line tool that helps you manage Jira tickets
more efficiently by integrating with Jira and Gemini APIs.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
}
