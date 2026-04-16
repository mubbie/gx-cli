package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).MarginBottom(1)
	sepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// PrintTable prints a styled table to stdout.
func PrintTable(headers []string, rows [][]string, title string) {
	PrintTableTo(os.Stdout, headers, rows, title)
}

// PrintTableTo prints a styled table to the given writer.
func PrintTableTo(w io.Writer, headers []string, rows [][]string, title string) {
	if title != "" {
		fmt.Fprintln(w, titleStyle.Render(title))
	}

	// Calculate column widths from VISIBLE content width (ANSI-aware)
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			vis := lipgloss.Width(cell)
			if i < len(widths) && vis > widths[i] {
				widths[i] = vis
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
	fmt.Fprintln(w, strings.Join(hParts, ""))

	// Separator
	sParts := make([]string, len(headers))
	for i := range headers {
		sParts[i] = sepStyle.Render(strings.Repeat("─", widths[i]))
	}
	fmt.Fprintln(w, strings.Join(sParts, ""))

	// Rows
	for _, row := range rows {
		parts := make([]string, len(headers))
		for i := range headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			parts[i] = lipgloss.NewStyle().Width(widths[i]).MaxWidth(widths[i]).Render(cell)
		}
		fmt.Fprintln(w, strings.Join(parts, ""))
	}
	fmt.Fprintln(w)
}
