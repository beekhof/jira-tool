package cmd

import (
	"fmt"

	"github.com/beekhof/jira-tool/pkg/gemini"

	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available Gemini models",
	Long:  `List all available Gemini models that support generateContent.`,
	RunE:  runModels,
}

func runModels(cmd *cobra.Command, args []string) error {
	configDir := GetConfigDir()
	
	models, err := gemini.ListModels(configDir)
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	fmt.Println("Available Gemini models that support generateContent:")
	fmt.Println()
	
	found := false
	for _, model := range models {
		for _, method := range model.SupportedMethods {
			if method == "generateContent" {
				fmt.Printf("  - %s\n", model.Name)
				if model.DisplayName != "" {
					fmt.Printf("    Display Name: %s\n", model.DisplayName)
				}
				found = true
				break
			}
		}
	}
	
	if !found {
		fmt.Println("  No models found that support generateContent")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}

