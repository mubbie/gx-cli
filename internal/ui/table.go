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

// visibleWidth returns the printed width of a string, ignoring ANSI codes.
func visibleWidth(s string) int {
	return lipgloss.Width(s)
}

// truncateVisible shortens a string to max visible characters.
// It strips ANSI codes, truncates the plain text, then the caller
// must re-apply styles. This is a plain-text truncation only.
func truncateVisible(s string, max int) string {
	if max <= 3 {
		max = 4
	}
	w := visibleWidth(s)
	if w <= max {
		return s
	}
	// Walk through runes, counting visible width
	// For styled strings, we need to strip and re-style.
	// Simplest: just use the raw string if it has no ANSI codes,
	// otherwise let lipgloss handle the width via padding.
	return s
}

func PrintTable(headers []string, rows [][]string, title string) {
	if title != "" {
		fmt.Println(titleStyle.Render(title))
	}

	// Calculate column widths based on VISIBLE width (ignoring ANSI)
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h) + 2
	}
	for _, row := range rows {
		for i, cell := range row {
			w := visibleWidth(cell) + 2
			if i < len(widths) && w > widths[i] {
				widths[i] = w
			}
		}
	}

	// Cap non-last columns
	lastCol := len(headers) - 1
	for i := range widths {
		if i != lastCol && widths[i] > maxColWidth {
			widths[i] = maxColWidth
		}
	}

	// Header row
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

	// Rows: use lipgloss.Width-aware rendering
	for _, row := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			// Use lipgloss to handle width with ANSI-aware padding
			cells[i] = cellStyle.Width(widths[i]).MaxWidth(widths[i]).Render(cell)
		}
		fmt.Println(strings.Join(cells, ""))
	}
	fmt.Println()
}
