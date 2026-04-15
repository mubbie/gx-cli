// Package ui provides terminal styling and output helpers.
package ui

import "github.com/charmbracelet/lipgloss"

var (
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	WarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	InfoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	DimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	BoldStyle    = lipgloss.NewStyle().Bold(true)
)
