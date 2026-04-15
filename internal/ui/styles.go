// Package ui provides terminal styling and output helpers.
package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Status
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	WarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	InfoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	DimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	BoldStyle    = lipgloss.NewStyle().Bold(true)

	// Content
	BranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)  // Cyan bold for branch names
	HashStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))             // Yellow for commit hashes
	AuthorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))             // Magenta for author names
	DateStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))           // Gray for dates
	FileStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))             // Blue for file paths
	AddStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))             // Green for additions
	DelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))             // Red for deletions

	// Labels
	LabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true) // Dim bold for labels like "Branch:", "Stash:"
	HeadMarker = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)   // Green bold for HEAD marker
)
