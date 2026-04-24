package model

import (
	"strings"

	"lazyssm/tui"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Model struct {
	Config           tui.Config
	State            tui.State
	RunningInstances []string
}

func InitModel(cfg *tui.Config) Model {
	return Model{
		Config: *cfg,
		State: nil
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "ctrl+z":
			return m, tea.Suspend
		}
	}

	return m, nil
}

func (m Model) View() tea.View {
	var s strings.Builder

	items := make([]list.Item, len(m.Config.Services))
	for i, serv := range m.Config.Services {
		items[i] = Item{title: serv.Name, description: serv.Profile}
	}

	defaultDelegate := list.NewDefaultDelegate()
	l := list.New(items, defaultDelegate, 0, 0)
	l.Title = "Test title"

	title := "lazyssm"
	cols := lipgloss.JoinHorizontal(
		lipgloss.Top,
		tui.Focused.Render(l.View()),
		tui.Border.Render("B"),
	)

	rows := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		cols,
	)

	s.WriteString(rows)

	v := tea.NewView(s.String())
	v.AltScreen = true

	return v
}
