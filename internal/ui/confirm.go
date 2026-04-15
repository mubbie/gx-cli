package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Confirm shows a yes/no prompt and returns true if the user confirms.
func Confirm(message string) bool {
	fmt.Printf("\n? %s [y/N] ", message)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}
