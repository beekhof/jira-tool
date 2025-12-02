package cmd

import (
	"fmt"
	"strings"

	"github.com/beekhof/jira-tool/pkg/gemini"

	"github.com/spf13/cobra"
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Show default prompt templates in YAML format",
	Long: `Display the default prompt templates in YAML format that can be copied into your config file.
These are the templates used when custom templates are not specified in the configuration.`,
	RunE: runTemplates,
}

func runTemplates(cmd *cobra.Command, args []string) error {
	templates := gemini.GetDefaultTemplates()

	fmt.Println("# Default prompt templates")
	fmt.Println("# Copy these into your config.yaml file to customize the prompts")
	fmt.Println()
	fmt.Println("question_prompt_template: |")
	fmt.Println(indentYAML(templates["question_prompt_template"]))
	fmt.Println()
	fmt.Println("description_prompt_template: |")
	fmt.Println(indentYAML(templates["description_prompt_template"]))
	fmt.Println()
	fmt.Println("spike_question_prompt_template: |")
	fmt.Println(indentYAML(templates["spike_question_prompt_template"]))
	fmt.Println()
	fmt.Println("spike_prompt_template: |")
	fmt.Println(indentYAML(templates["spike_prompt_template"]))

	return nil
}

// indentYAML indents each line of the string by 2 spaces for YAML literal block
func indentYAML(s string) string {
	lines := strings.Split(s, "\n")
	var result strings.Builder
	for _, line := range lines {
		if line == "" {
			result.WriteString("  \n")
		} else {
			result.WriteString("  ")
			result.WriteString(line)
			result.WriteString("\n")
		}
	}
	return strings.TrimRight(result.String(), "\n")
}

func init() {
	utilsCmd.AddCommand(templatesCmd)
}
