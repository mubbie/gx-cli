package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mubbie/gx-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Update gx to the latest version",
		RunE:  runUpdate,
	})
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Current version: %s\n", appVersion)

	method := detectInstallMethod()

	switch method {
	case "homebrew":
		fmt.Println("Installed via Homebrew. Updating...")
		fmt.Println()
		c := exec.Command("brew", "upgrade", "gx-git")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			ui.PrintError("Update failed. Try: brew upgrade gx-git")
			return nil
		}
		ui.PrintSuccess("Updated via Homebrew. Run `gx --version` to verify.")
	default:
		ui.PrintInfo("To update, reinstall via your package manager:")
		fmt.Println("  Homebrew: brew upgrade gx-git")
		fmt.Println("  pipx:    pipx upgrade gx-git")
		fmt.Println("  go:      go install github.com/mubbie/gx-cli@latest")
	}
	return nil
}

func detectInstallMethod() string {
	exe, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}
	lower := strings.ToLower(resolved)
	if strings.Contains(lower, "cellar") || strings.Contains(lower, "homebrew") || strings.Contains(lower, "linuxbrew") {
		return "homebrew"
	}
	return "unknown"
}
