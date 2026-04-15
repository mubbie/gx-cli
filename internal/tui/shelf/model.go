package shelf

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mubbie/gx-cli/internal/git"
)

type StashEntry struct {
	Index   int
	ID      string
	Time    string
	Message string
	Branch  string
}

func (s StashEntry) Title() string       { return s.ID + "  " + s.Time }
func (s StashEntry) Description() string {
	msg := s.Message
	if len(msg) > 60 {
		msg = msg[:57] + "..."
	}
	return msg
}
func (s StashEntry) FilterValue() string { return s.Message + " " + s.Branch }

type diffLoadedMsg struct {
	id   string
	diff string
}

type Action struct {
	Type string // "pop", "apply", "drop", ""
	ID   string
}

type Model struct {
	list      list.Model
	viewport  viewport.Model
	stashes   []StashEntry
	diffCache map[string]string
	width     int
	height    int
	focused   int // 0=list, 1=viewport
	action    Action
	quitting  bool
	ready     bool
}

var (
	listStyle     = lipgloss.NewStyle().Padding(1, 2)
	viewportStyle = lipgloss.NewStyle().Padding(1, 2).BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Padding(0, 2)
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).Padding(0, 2)
)

func New(stashes []StashEntry) Model {
	items := make([]list.Item, len(stashes))
	for i, s := range stashes {
		items[i] = s
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	l := list.New(items, delegate, 0, 0)
	l.Title = "gx shelf"
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)
	l.SetShowHelp(false)

	vp := viewport.New(0, 0)
	vp.SetContent("Select a stash to preview its diff")

	return Model{
		list:      l,
		viewport:  vp,
		stashes:   stashes,
		diffCache: make(map[string]string),
	}
}

func (m Model) Init() tea.Cmd {
	if len(m.stashes) > 0 {
		return loadDiff(m.stashes[0].ID)
	}
	return nil
}

func loadDiff(stashID string) tea.Cmd {
	return func() tea.Msg {
		diff, err := git.Run("stash", "show", "-p", stashID)
		if err != nil {
			return diffLoadedMsg{id: stashID, diff: "(failed to load diff)"}
		}
		return diffLoadedMsg{id: stashID, diff: diff}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listWidth := msg.Width * 40 / 100
		vpWidth := msg.Width - listWidth - 3
		m.list.SetSize(listWidth, msg.Height-2)
		m.viewport = viewport.New(vpWidth, msg.Height-4)
		m.ready = true
		// Reload current diff
		if item, ok := m.list.SelectedItem().(StashEntry); ok {
			if cached, ok := m.diffCache[item.ID]; ok {
				m.viewport.SetContent(colorDiff(cached))
			} else {
				return m, loadDiff(item.ID)
			}
		}
		return m, nil

	case diffLoadedMsg:
		m.diffCache[msg.id] = msg.diff
		if item, ok := m.list.SelectedItem().(StashEntry); ok {
			if item.ID == msg.id {
				m.viewport.SetContent(colorDiff(msg.diff))
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(StashEntry); ok {
				m.action = Action{Type: "pop", ID: item.ID}
				m.quitting = true
				return m, tea.Quit
			}
		case " ":
			if item, ok := m.list.SelectedItem().(StashEntry); ok {
				m.action = Action{Type: "apply", ID: item.ID}
				m.quitting = true
				return m, tea.Quit
			}
		case "d":
			if !m.list.SettingFilter() {
				if item, ok := m.list.SelectedItem().(StashEntry); ok {
					m.action = Action{Type: "drop", ID: item.ID}
					m.quitting = true
					return m, tea.Quit
				}
			}
		case "tab":
			m.focused = 1 - m.focused
			return m, nil
		}
	}

	var cmds []tea.Cmd

	if m.focused == 0 {
		prevItem := m.list.SelectedItem()
		newList, cmd := m.list.Update(msg)
		m.list = newList
		cmds = append(cmds, cmd)

		// If selection changed, load new diff
		newItem := m.list.SelectedItem()
		if prevItem != newItem {
			if entry, ok := newItem.(StashEntry); ok {
				if cached, ok := m.diffCache[entry.ID]; ok {
					m.viewport.SetContent(colorDiff(cached))
				} else {
					cmds = append(cmds, loadDiff(entry.ID))
				}
			}
		}
	} else {
		newVP, cmd := m.viewport.Update(msg)
		m.viewport = newVP
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.quitting || !m.ready {
		return ""
	}

	listView := listStyle.Width(m.width * 40 / 100).Height(m.height - 2).Render(m.list.View())
	vpView := viewportStyle.Width(m.width - m.width*40/100 - 3).Height(m.height - 2).Render(m.viewport.View())

	main := lipgloss.JoinHorizontal(lipgloss.Top, listView, vpView)
	help := helpStyle.Render("enter pop | space apply | d drop | / filter | tab focus | esc quit")

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

func (m Model) Result() Action { return m.action }

func colorDiff(diff string) string {
	var lines []string
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	hunkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff "):
			lines = append(lines, headerStyle.Render(line))
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			lines = append(lines, lipgloss.NewStyle().Bold(true).Render(line))
		case strings.HasPrefix(line, "@@"):
			lines = append(lines, hunkStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			lines = append(lines, addStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			lines = append(lines, delStyle.Render(line))
		default:
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// Unused import guard
var _ = fmt.Sprintf
