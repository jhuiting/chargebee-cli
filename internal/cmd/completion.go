package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// newCompletionCmd creates the completion command that generates shell completion scripts.
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [shell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for the Chargebee CLI.

To load completions:

Bash:
  $ source <(cb completion bash)
  # Or add to ~/.bashrc:
  $ cb completion bash > /etc/bash_completion.d/cb

Zsh:
  $ source <(cb completion zsh)
  # Or add to fpath:
  $ cb completion zsh > "${fpath[1]}/_cb"

Fish:
  $ cb completion fish | source
  # Or persist:
  $ cb completion fish > ~/.config/fish/completions/cb.fish

PowerShell:
  PS> cb completion powershell | Out-String | Invoke-Expression
`,
	}

	cmd.AddCommand(newCompletionBashCmd())
	cmd.AddCommand(newCompletionZshCmd())
	cmd.AddCommand(newCompletionFishCmd())
	cmd.AddCommand(newCompletionPowershellCmd())

	return cmd
}

func newCompletionBashCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenBashCompletionV2(os.Stdout, true)
		},
	}
}

func newCompletionZshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenZshCompletion(os.Stdout)
		},
	}
}

func newCompletionFishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		},
	}
}

func newCompletionPowershellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "powershell",
		Short: "Generate PowerShell completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		},
	}
}
