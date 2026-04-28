package model

import (
	"log/slog"
	"slices"
	"strings"
	"time"

	"lazyssm/process"
	"lazyssm/tui"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var deleteKey = key.NewBinding(
	key.WithKeys("ctrl+d"),
	key.WithHelp("ctrl+d", "delete"),
)

type (
	refreshMsg     time.Time
	clearStatusMsg struct{}
)

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func tickRefresh() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return refreshMsg(t)
	})
}

type Model struct {
	Config           tui.Config
	State            tui.State
	RunningInstances map[string]*process.Proc
	pendingDelete    bool
}

func InitModel(cfg *tui.Config) Model {
	state := tui.NewState()

	for _, srv := range cfg.Services {
		state.ServiceItems = append(state.ServiceItems, tui.NewItem(&srv))
	}

	state.ServiceList = list.New(
		state.ServiceItems,
		list.NewDefaultDelegate(),
		state.UserInterface.PanelW,
		state.UserInterface.ListH,
	)
	state.ServiceList.Title = "available services"

	runningDelegate := process.RunningDelegate{
		ShortHelpFunc: func() []key.Binding { return []key.Binding{deleteKey} },
		FullHelpFunc:  func() [][]key.Binding { return [][]key.Binding{{deleteKey}} },
	}

	state.RunningList = list.New(
		state.RunningItems,
		runningDelegate,
		state.UserInterface.PanelW,
		state.UserInterface.ListH,
	)
	state.RunningList.Title = "running services"
	state.RunningList.DisableQuitKeybindings()
	state.RunningList.KeyMap.Filter = key.NewBinding(
		key.WithDisabled(),
	)

	return Model{
		Config:           *cfg,
		State:            state,
		RunningInstances: make(map[string]*process.Proc),
	}
}

func (m Model) Init() tea.Cmd {
	return tickRefresh()
}

func (m Model) Cleanup() {
	for name, p := range m.RunningInstances {
		snapshot := p.Snapshot()
		slog.Info("cleaning up process", "name", name, "pid", snapshot.PID, "status", snapshot.Status)
		p.Kill()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.State.UserInterface.Resize(msg.Width, msg.Height)
		m.State.ServiceList.SetSize(m.State.UserInterface.PanelW, m.State.UserInterface.ListH)
		m.State.RunningList.SetSize(m.State.UserInterface.PanelW, m.State.UserInterface.ListH)

	case clearStatusMsg:
		cmd := m.State.RunningList.NewStatusMessage("")
		return m, cmd

	case refreshMsg:
		m.refreshRunningItems()
		return m, tickRefresh()

	case tea.KeyPressMsg:
		switch {
		case msg.String() == "ctrl+c" || (msg.String() == "q" && !m.pendingDelete):
			m.Cleanup()
			return m, tea.Quit
		case msg.String() == "ctrl+z":
			return m, tea.Suspend
		case msg.String() == "tab":
			if m.pendingDelete {
				break
			}
			m.State.CircleActivePanel()
		case key.Matches(msg, deleteKey):
			if m.State.ActivePanel == "running" && len(m.State.RunningList.Items()) > 0 {
				m.pendingDelete = true
				m.State.RunningList.StatusMessageLifetime = time.Hour
				cmd := m.State.RunningList.NewStatusMessage("press enter to confirm, esc to cancel")
				return m, cmd
			}
		case msg.String() == "escape":
			if m.pendingDelete {
				m.pendingDelete = false
				m.State.RunningList.StatusMessageLifetime = time.Second
				cmd := m.State.RunningList.NewStatusMessage("")
				return m, cmd
			}
		case msg.String() == "enter":
			if m.pendingDelete && m.State.ActivePanel == "running" {
				m.pendingDelete = false
				selection := m.State.RunningList.SelectedItem()
				if selection != nil {
					idx := m.State.RunningList.GlobalIndex()
					name := selection.FilterValue()
					if p, ok := m.RunningInstances[name]; ok {
						snapshot := p.Snapshot()
						slog.Info("removing process from panel", "name", name, "pid", snapshot.PID, "status", snapshot.Status)
						p.Kill()
						delete(m.RunningInstances, name)
					}
					m.State.RunningList.RemoveItem(idx)
					m.State.RunningList.Select(max(idx-1, 0))
					statusCmd := m.State.RunningList.NewStatusMessage("deleted " + name)
					if len(m.RunningInstances) == 0 {
						m.State.SetActivePanel("services")
					}
					return m, tea.Batch(statusCmd, clearStatusAfter(2*time.Second))
				}
			}
			if m.State.ActivePanel == "services" {
				selection := m.State.ServiceList.SelectedItem()
				selectedItem, ok := selection.(tui.Item)
				if !ok {
					slog.Error("selected service item has unexpected type")
					return m, nil
				}

				p := &process.Proc{
					Name:          selectedItem.Title(),
					ProcessLogDir: m.Config.ProcessLogDir,
				}
				p.Run()
				m.RunningInstances[selectedItem.Title()] = p
				m.State.SetActivePanel("running")

				cmd := m.State.RunningList.InsertItem(len(m.State.RunningList.Items()), process.NewItem(p))

				return m, cmd
			}
		}
	}

	var cmd tea.Cmd
	if m.State.ActivePanel == "services" {
		m.State.ServiceList, cmd = m.State.ServiceList.Update(msg)
	}
	if m.State.ActivePanel == "running" {
		m.State.RunningList, cmd = m.State.RunningList.Update(msg)
	}

	return m, cmd
}

func (m *Model) refreshRunningItems() {
	names := make([]string, 0, len(m.RunningInstances))
	for name := range m.RunningInstances {
		names = append(names, name)
	}
	slices.Sort(names)

	items := make([]list.Item, 0, len(m.RunningInstances))
	for _, name := range names {
		p := m.RunningInstances[name]
		p.Refresh()
		items = append(items, process.NewItem(p))
	}
	m.State.RunningItems = items
	m.State.RunningList.SetItems(items)
}

func (m Model) View() tea.View {
	var s strings.Builder
	var services string
	var running string

	panelW := m.State.UserInterface.PanelW
	panelH := m.State.UserInterface.PanelH

	servicesFocused := m.State.ActivePanel == "services"
	services = tui.PanelStyle(panelW, panelH, servicesFocused).Render(m.State.ServiceList.View())
	var runningContent string
	if len(m.State.RunningList.Items()) > 0 {
		runningContent = m.State.RunningList.View()
	} else {
		runningContent = ""
	}
	running = tui.PanelStyle(panelW, panelH, !servicesFocused).Render(runningContent)

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
