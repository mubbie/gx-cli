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

// Plural returns "s" when n != 1, empty string otherwise.
func Plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// PluralES returns "es" when n != 1, empty string otherwise.
func PluralES(n int) string {
	if n == 1 {
		return ""
	}
	return "es"
}

// PluralIES returns "ies" when n != 1, "y" otherwise.
func PluralIES(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
