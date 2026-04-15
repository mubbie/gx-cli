package switcher

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mubbie/gx-cli/internal/git"
)

type BranchItem struct {
	Name   string
	Date   string
	Author string
}

func (b BranchItem) Title() string { return b.Name }
func (b BranchItem) Description() string {
	return fmt.Sprintf("%s  %s", git.TimeAgo(b.Date), b.Author)
}
func (b BranchItem) FilterValue() string { return b.Name }

type Model struct {
	list     list.Model
	selected string
	quitting bool
	width    int
	height   int
}

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)
)

func New(branches []BranchItem) Model {
	items := make([]list.Item, len(branches))
	for i, b := range branches {
		items[i] = b
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("6")).Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("240"))

	l := list.New(items, delegate, 0, 0)
	l.Title = "Switch Branch"
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)
	l.SetShowHelp(true)

	return Model{list: l}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(BranchItem); ok {
				m.selected = item.Name
				m.quitting = true
				return m, tea.Quit
			}
		case "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	return appStyle.Render(m.list.View())
}

func (m Model) Selected() string {
	return m.selected
}
