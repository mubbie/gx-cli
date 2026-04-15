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
  nuke, update

Run gx <command> --help for details.
`

// SetVersion sets the app version from main.go.
func SetVersion(v string) {
	appVersion = v
	rootCmd.Version = v
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
