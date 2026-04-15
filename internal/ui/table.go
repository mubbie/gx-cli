package ui

import (
	"fmt"
	"strings"
)

// PrintTable prints a simple formatted table.
func PrintTable(headers []string, rows [][]string, title string) {
	if title != "" {
		fmt.Println(BoldStyle.Render(title))
		fmt.Println()
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	headerParts := make([]string, len(headers))
	for i, h := range headers {
		headerParts[i] = fmt.Sprintf("%-*s", widths[i], h)
	}
	fmt.Println("  " + strings.Join(headerParts, "  "))

	// Print separator
	sepParts := make([]string, len(headers))
	for i := range headers {
		sepParts[i] = strings.Repeat("-", widths[i])
	}
	fmt.Println("  " + strings.Join(sepParts, "  "))

	// Print rows
	for _, row := range rows {
		parts := make([]string, len(headers))
		for i := range headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			parts[i] = fmt.Sprintf("%-*s", widths[i], cell)
		}
		fmt.Println("  " + strings.Join(parts, "  "))
	}
}
