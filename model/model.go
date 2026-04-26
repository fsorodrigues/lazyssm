package model

import (
	"log"
	"maps"
	"slices"
	"strings"

	"lazyssm/process"
	"lazyssm/tui"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Model struct {
	Config           tui.Config
	State            tui.State
	RunningInstances map[string]*process.Proc
}

func InitModel(cfg *tui.Config) Model {
	state := tui.NewState()

	for _, srv := range cfg.Services {
		state.Items = append(state.Items, tui.NewItem(&srv))
	}

	state.List = list.New(
		state.Items,
		list.NewDefaultDelegate(),
		state.UserInterface.PanelW,
		state.UserInterface.ListH,
	)
	state.List.Title = "available services"

	return Model{
		Config:           *cfg,
		State:            state,
		RunningInstances: make(map[string]*process.Proc),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Cleanup() {
	for name, p := range m.RunningInstances {
		log.Printf("Cleaning up process: %s (PID %d)\n", name, p.PID)
		p.Kill()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Printf("Update: %s", msg)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		log.Printf("Update: %+v", msg)
		m.State.UserInterface.Resize(msg.Width, msg.Height)
		m.State.List.SetSize(m.State.UserInterface.PanelW, m.State.UserInterface.ListH)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Cleanup()
			return m, tea.Quit
		case "ctrl+z":
			return m, tea.Suspend
		case "tab":
			m.State.CircleActivePanel()
		case "enter":
			if m.State.ActivePanel == "services" {
				selection := m.State.List.SelectedItem()
				selectedItem, ok := selection.(tui.Item)
				if !ok {
					log.Print("Error")
				}

				p := &process.Proc{
					Name: selectedItem.Title(),
				}
				p.Run()
				m.RunningInstances[selectedItem.Title()] = p
				m.State.SetActivePanel("running")
			}
		}
	}

	var cmd tea.Cmd
	m.State.List, cmd = m.State.List.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	var s strings.Builder
	var services string
	var running string

	panelW := m.State.UserInterface.PanelW
	panelH := m.State.UserInterface.PanelH

	servicesFocused := m.State.ActivePanel == "services"
	services = tui.PanelStyle(panelW, panelH, servicesFocused).Render(m.State.List.View())
	running = tui.PanelStyle(panelW, panelH, !servicesFocused).Render(slices.Collect(maps.Keys(m.RunningInstances))...)

	cols := lipgloss.JoinHorizontal(
		lipgloss.Top,
		services,
		running,
	)

	rows := lipgloss.JoinVertical(
		lipgloss.Left,
		tui.TitleStyle().Render("lazyssm"),
		cols,
	)

	s.WriteString(rows)

	v := tea.NewView(s.String())
	v.AltScreen = true

	return v
}
