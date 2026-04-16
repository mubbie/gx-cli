package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).MarginBottom(1)
)

const maxColWidth = 50

func PrintTable(headers []string, rows [][]string, title string) {
	if title != "" {
		fmt.Println(titleStyle.Render(title))
	}

	// Calculate column widths based on content
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h) + 2
	}
	for _, row := range rows {
		for i, cell := range row {
			w := lipgloss.Width(cell) + 2
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

	// Build columns
	cols := make([]table.Column, len(headers))
	for i, h := range headers {
		cols[i] = table.Column{Title: h, Width: widths[i]}
	}

	// Build rows
	tableRows := make([]table.Row, len(rows))
	for i, row := range rows {
		r := make(table.Row, len(headers))
		for j := range headers {
			if j < len(row) {
				r[j] = row[j]
			}
		}
		tableRows[i] = r
	}

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		Bold(true).
		Foreground(lipgloss.Color("4")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("240"))
	s.Selected = lipgloss.NewStyle() // Disable selection highlighting for static display
	s.Cell = s.Cell.Padding(0, 1)

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(tableRows),
		table.WithHeight(len(tableRows)+1),
		table.WithStyles(s),
	)

	fmt.Println(t.View())
	fmt.Println()
}
