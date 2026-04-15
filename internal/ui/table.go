package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4")).Padding(0, 1)
	cellStyle   = lipgloss.NewStyle().Padding(0, 1)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).MarginBottom(1)
)

const maxColWidth = 50

// truncate shortens s to max characters, appending "..." if truncated.
func truncate(s string, max int) string {
	if max <= 3 {
		max = 4
	}
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func PrintTable(headers []string, rows [][]string, title string) {
	if title != "" {
		fmt.Println(titleStyle.Render(title))
	}

	lastCol := len(headers) - 1

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h) + 2 // padding
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell)+2 > widths[i] {
				widths[i] = len(cell) + 2
			}
		}
	}

	// Cap non-last columns at maxColWidth
	for i := range widths {
		if i != lastCol && widths[i] > maxColWidth {
			widths[i] = maxColWidth
		}
	}

	// Build header row
	headerCells := make([]string, len(headers))
	for i, h := range headers {
		headerCells[i] = headerStyle.Width(widths[i]).Render(h)
	}
	fmt.Println(strings.Join(headerCells, ""))

	// Separator – matches actual column widths
	sepParts := make([]string, len(headers))
	for i := range headers {
		sepParts[i] = DimStyle.Render(strings.Repeat("─", widths[i]))
	}
	fmt.Println(strings.Join(sepParts, ""))

	// Rows
	for _, row := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			// Truncate non-last columns that exceed width (minus 2 for padding)
			if i != lastCol {
				cell = truncate(cell, widths[i]-2)
			}
			cells[i] = cellStyle.Width(widths[i]).Render(cell)
		}
		fmt.Println(strings.Join(cells, ""))
	}
	fmt.Println()
}
