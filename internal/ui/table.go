package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).MarginBottom(1)
	sepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func PrintTable(headers []string, rows [][]string, title string) {
	if title != "" {
		fmt.Println(titleStyle.Render(title))
	}

	// Calculate column widths from VISIBLE content width (ANSI-aware)
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			w := lipgloss.Width(cell)
			if i < len(widths) && w > widths[i] {
				widths[i] = w
			}
		}
	}

	// Add padding
	for i := range widths {
		widths[i] += 2
	}

	// Header
	hParts := make([]string, len(headers))
	for i, h := range headers {
		hParts[i] = headerStyle.Width(widths[i]).Render(h)
	}
	fmt.Println(strings.Join(hParts, ""))

	// Separator
	sParts := make([]string, len(headers))
	for i := range headers {
		sParts[i] = sepStyle.Render(strings.Repeat("─", widths[i]))
	}
	fmt.Println(strings.Join(sParts, ""))

	// Rows - use lipgloss Width for ANSI-aware padding, MaxWidth for truncation
	for _, row := range rows {
		parts := make([]string, len(headers))
		for i := range headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			parts[i] = lipgloss.NewStyle().Width(widths[i]).MaxWidth(widths[i]).Render(cell)
		}
		fmt.Println(strings.Join(parts, ""))
	}
	fmt.Println()
}
