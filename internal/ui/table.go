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

func PrintTable(headers []string, rows [][]string, title string) {
	if title != "" {
		fmt.Println(titleStyle.Render(title))
	}

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

	// Build header row
	headerCells := make([]string, len(headers))
	for i, h := range headers {
		headerCells[i] = headerStyle.Width(widths[i]).Render(h)
	}
	fmt.Println(strings.Join(headerCells, ""))

	// Separator
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
			cells[i] = cellStyle.Width(widths[i]).Render(cell)
		}
		fmt.Println(strings.Join(cells, ""))
	}
	fmt.Println()
}
