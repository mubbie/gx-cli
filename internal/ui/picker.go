package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ReadLine reads a single line from stdin with a "> " prompt.
func ReadLine() string {
	fmt.Print("> ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}
