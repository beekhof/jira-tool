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

	// Register completion command under utils instead of root
	// Disable the default completion command and create our own
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: `Generate the autocompletion script for the specified shell.
See each sub-command's help for details on how to add the autocompletion to your shell.`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	}

	// Add the standard completion subcommands
	bashCmd := &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		Long: `Generate the bash autocompletion script for jira-tool.

To load completions in your current shell session:
  source <(jira utils completion bash)

To load completions for all new sessions, execute once:
  Linux:
    jira utils completion bash > /etc/bash_completion.d/jira-tool
  macOS:
    jira utils completion bash > /usr/local/etc/bash_completion.d/jira-tool
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return rootCmd.GenBashCompletion(cmd.OutOrStdout())
		},
	}

	zshCmd := &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		Long: `Generate the zsh autocompletion script for jira-tool.

To load completions in your current shell session:
  source <(jira utils completion zsh)

To load completions for all new sessions, add to your ~/.zshrc:
  echo 'source <(jira utils completion zsh)' >> ~/.zshrc
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		},
	}

	fishCmd := &cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		Long: `Generate the fish autocompletion script for jira-tool.

To load completions in your current shell session:
  jira utils completion fish | source

To load completions for all new sessions, execute once:
  jira utils completion fish > ~/.config/fish/completions/jira-tool.fish
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		},
	}

	powershellCmd := &cobra.Command{
		Use:   "powershell",
		Short: "Generate powershell completion script",
		Long: `Generate the powershell autocompletion script for jira-tool.

To load completions in your current shell session:
  jira utils completion powershell | Out-String | Invoke-Expression

To load completions for all new sessions, add to your PowerShell profile:
  jira utils completion powershell >> $PROFILE
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return rootCmd.GenPowerShellCompletion(cmd.OutOrStdout())
		},
	}

	completionCmd.AddCommand(bashCmd, zshCmd, fishCmd, powershellCmd)
	utilsCmd.AddCommand(completionCmd)
}
