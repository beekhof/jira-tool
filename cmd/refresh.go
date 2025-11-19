package cmd

import (
	"fmt"
	"os"

	"github.com/beekhof/jira-tool/pkg/jira"

	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh cached data",
	Long:  `Clear the local cache to force fresh data from Jira on the next run.`,
	RunE:  runRefresh,
}

func runRefresh(cmd *cobra.Command, args []string) error {
	configDir := GetConfigDir()
	cachePath := jira.GetCachePath(configDir)

	// Check if cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		fmt.Println("Cache is already empty.")
		return nil
	}

	// Delete the cache file
	if err := os.Remove(cachePath); err != nil {
		return fmt.Errorf("failed to delete cache file: %w", err)
	}

	fmt.Println("Cache cleared.")
	return nil
}

func init() {
	utilsCmd.AddCommand(refreshCmd)
}
