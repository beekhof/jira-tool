package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	configDir string
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

// GetConfigDir returns the configured config directory, or the default
func GetConfigDir() string {
	if configDir != "" {
		return configDir
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".jira-tool"
	}
	return filepath.Join(homeDir, ".jira-tool")
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "", "Configuration directory (default: ~/.jira-tool)")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(estimateCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(acceptCmd)
	rootCmd.AddCommand(refreshCmd)
}
