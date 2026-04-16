// Package cmd implements all gx CLI commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var appVersion = "dev"

var rootCmd = &cobra.Command{
	Use:   "gx",
	Short: "gx: Git Productivity Toolkit",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(groupedHelp)
	},
}

const groupedHelp = `gx: Git Productivity Toolkit

Setup:
  init

Everyday:
  undo, redo, oops, switch, context, sweep, shelf

Insight:
  who, recap, drift, conflicts, handoff, view

Stacking:
  stack, sync, retarget, graph, up, down, top, bottom, parent

Utility:
  nuke, update, completion

Run gx <command> --help for details.
`

// SetVersion sets the app version from main.go.
func SetVersion(v string) {
	appVersion = v
	rootCmd.Version = v
}

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for gx.

To load completions:

  bash:
    source <(gx completion bash)

  zsh:
    gx completion zsh > "${fpath[1]}/_gx"

  fish:
    gx completion fish | source

  powershell:
    gx completion powershell | Out-String | Invoke-Expression`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletion(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	})
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
