package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/spf13/cobra"
)

var (
	configDir  string
	noCache    bool
	filterFlag string
	noFilterFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "jira",
	Short: "A CLI tool to streamline Jira workflows",
	Long: `jira-tool is a command-line tool that helps you manage Jira tickets
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

// GetNoCache returns whether the --no-cache flag is set
func GetNoCache() bool {
	return noCache
}

// GetTicketFilter returns the active ticket filter based on precedence:
// --no-filter > --filter (command-line) > ticket_filter (config)
func GetTicketFilter(cfg *config.Config) string {
	// If --no-filter is set, return empty string (bypass all filters)
	if noFilterFlag {
		return ""
	}
	// If --filter flag is set, use it (overrides config)
	if filterFlag != "" {
		return filterFlag
	}
	// Otherwise, use config filter if set
	if cfg != nil && cfg.TicketFilter != "" {
		return cfg.TicketFilter
	}
	// No filter set
	return ""
}

// normalizeTicketID prepends the default project if the ticket ID doesn't have a project prefix
// Example: "353" with default project "OCPNAS" becomes "OCPNAS-353"
// Example: "OCPNAS-353" remains "OCPNAS-353"
func normalizeTicketID(ticketID, defaultProject string) string {
	// If ticket ID already contains a dash, it has a project prefix
	if strings.Contains(ticketID, "-") {
		return ticketID
	}

	// If no default project configured, return as-is
	if defaultProject == "" {
		return ticketID
	}

	// Prepend default project
	return defaultProject + "-" + ticketID
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "", "Configuration directory (default: ~/.jira-tool)")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "Bypass cache and fetch fresh data from API")
	rootCmd.PersistentFlags().StringVar(&filterFlag, "filter", "", "JQL filter to append to all ticket queries")
	rootCmd.PersistentFlags().BoolVar(&noFilterFlag, "no-filter", false, "Bypass ticket filter (overrides --filter and config)")
	// Commands register themselves in their own init() functions
}
