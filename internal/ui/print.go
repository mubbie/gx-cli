package ui

import "fmt"

// PrintSuccess prints a green success message.
func PrintSuccess(msg string) {
	fmt.Println(SuccessStyle.Render("OK") + " " + msg)
}

// PrintError prints a red error message.
func PrintError(msg string) {
	fmt.Println(ErrorStyle.Render("ERROR") + " " + msg)
}

// PrintWarning prints a yellow warning message.
func PrintWarning(msg string) {
	fmt.Println(WarningStyle.Render("WARN") + " " + msg)
}

// PrintInfo prints a blue info message.
func PrintInfo(msg string) {
	fmt.Println(InfoStyle.Render(">") + " " + msg)
}

// PrintDryRun prints the dry-run header and a list of actions.
func PrintDryRun(actions []string) {
	fmt.Println()
	fmt.Println(WarningStyle.Bold(true).Render("DRY RUN. No changes will be made"))
	fmt.Println()
	for _, a := range actions {
		fmt.Println("  " + a)
	}
	fmt.Println()
}
