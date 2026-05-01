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

const (
	authOutputLimitBytes = 64 * 1024
	authModalMinVisible  = 1200 * time.Millisecond
)

var deleteKey = key.NewBinding(
	key.WithKeys("ctrl+d"),
	key.WithHelp("ctrl+d", "delete"),
)

type (
	refreshMsg            time.Time
	clearStatusMsg        struct{}
	authSessionStartedMsg struct {
		service tui.Service
		session *process.AuthPTYSession
		err     error
	}
	authOutputMsg struct {
		chunk []byte
		ok    bool
	}
	authExitMsg struct {
		err error
	}
	authSuccessProceedMsg struct{}
	authInputErrMsg       struct {
		err error
	}
)

type authModalState struct {
	active          bool
	commandLabel    string
	selectedService tui.Service

	session *process.AuthPTYSession
	output  []byte
	started time.Time

	outputClosed bool
	exitReceived bool
	exitErr      error
}

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

func startAuthSessionCmd(command string, service tui.Service) tea.Cmd {
	return func() tea.Msg {
		session, err := process.StartAuthPTYSession(command)
		return authSessionStartedMsg{service: service, session: session, err: err}
	}
}

func waitAuthOutputCmd(ch <-chan []byte) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-ch
		return authOutputMsg{chunk: chunk, ok: ok}
	}
}

func waitAuthExitCmd(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-ch
		if !ok {
			err = nil
		}
		return authExitMsg{err: err}
	}
}

func writeAuthInputCmd(s *process.AuthPTYSession, b []byte) tea.Cmd {
	return func() tea.Msg {
		return authInputErrMsg{err: s.WriteInput(b)}
	}
}

func authSuccessProceedCmd(d time.Duration) tea.Cmd {
	if d <= 0 {
		return func() tea.Msg { return authSuccessProceedMsg{} }
	}

	return tea.Tick(d, func(time.Time) tea.Msg {
		return authSuccessProceedMsg{}
	})
}

type Model struct {
	Config           tui.Config
	State            tui.State
	RunningInstances map[string]*process.Proc
	CommandBuilder   process.CommandBuilder
	AuthCommand      string
	SkipAuth         bool
	Simulate         bool
	pendingDelete    bool
	startInProgress  bool
	authModal        authModalState
}

func InitModel(cfg *tui.Config, builder process.CommandBuilder, authCommand string, skipAuth bool, simulate bool) Model {
	state := tui.NewState()

	for _, srv := range cfg.Services {
		state.ServiceItems = append(state.ServiceItems, tui.NewItem(&srv))
	}

	state.ServiceList = list.New(
		state.ServiceItems,
		tui.ServiceDelegate{},
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
		CommandBuilder:   builder,
		AuthCommand:      authCommand,
		SkipAuth:         skipAuth,
		Simulate:         simulate,
	}
}

func (m Model) Init() tea.Cmd {
	return tickRefresh()
}

func (m Model) Cleanup() {
	if m.authModal.session != nil {
		_ = m.authModal.session.Interrupt()
		_ = m.authModal.session.Close()
	}

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

		if m.authModal.active {
			_ = m.resizeAuthPTYToModal()
		}

	case clearStatusMsg:
		if m.State.ActivePanel == "services" {
			cmd := m.State.ServiceList.NewStatusMessage("")
			return m, cmd
		}
		cmd := m.State.RunningList.NewStatusMessage("")
		return m, cmd

	case authSessionStartedMsg:
		if msg.err != nil {
			m.startInProgress = false
			m.closeAuthModal()
			slog.Warn("aws-mfa preflight failed", "error", msg.err)
			cmd := m.State.ServiceList.NewStatusMessage("aws-mfa failed")
			return m, tea.Batch(cmd, clearStatusAfter(2*time.Second))
		}

		m.authModal.active = true
		m.authModal.session = msg.session
		_ = m.resizeAuthPTYToModal()

		return m, tea.Batch(
			waitAuthOutputCmd(msg.session.Output()),
			waitAuthExitCmd(msg.session.Done()),
		)

	case authOutputMsg:
		if !msg.ok {
			m.authModal.outputClosed = true
			return m, nil
		}

		m.appendAuthOutput(msg.chunk)
		if m.authModal.session == nil {
			return m, nil
		}

		return m, waitAuthOutputCmd(m.authModal.session.Output())

	case authExitMsg:
		m.authModal.exitReceived = true
		m.authModal.exitErr = msg.err

		if m.authModal.session != nil {
			_ = m.authModal.session.Close()
		}

		if msg.err != nil {
			m.startInProgress = false
			m.closeAuthModal()
			cmd := m.State.ServiceList.NewStatusMessage("aws-mfa failed")
			return m, tea.Batch(cmd, clearStatusAfter(2*time.Second))
		}

		m.appendAuthOutput([]byte("\r\naws-mfa succeeded\r\n"))
		wait := authModalMinVisible - time.Since(m.authModal.started)
		return m, authSuccessProceedCmd(wait)

	case authSuccessProceedMsg:
		if !m.authModal.active {
			return m, nil
		}
		return m.finishAuthSuccess()

	case authInputErrMsg:
		if msg.err != nil {
			slog.Debug("write auth PTY input", "error", msg.err)
		}
		return m, nil

	case refreshMsg:
		m.refreshRunningItems()
		return m, tickRefresh()

	case tea.KeyPressMsg:
		if m.authModal.active {
			return m.handleAuthModalKey(msg)
		}

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
				if m.startInProgress || m.authModal.active {
					return m, nil
				}

				selection := m.State.ServiceList.SelectedItem()
				selectedItem, ok := selection.(tui.Item)
				if !ok {
					slog.Error("selected service item has unexpected type")
					return m, nil
				}

				if m.shouldRunAuthPreflight() {
					selected := *selectedItem.Service
					m.startInProgress = true
					m.initAuthModalSkeleton(selected, m.AuthCommand)
					return m, startAuthSessionCmd(m.AuthCommand, selected)
				}

				return m.startService(*selectedItem.Service)
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

func (m Model) shouldRunAuthPreflight() bool {
	if m.Simulate {
		return false
	}
	if m.SkipAuth {
		return false
	}
	return true
}

func (m Model) startService(service tui.Service) (tea.Model, tea.Cmd) {
	p := &process.Proc{
		Name:          service.Name,
		Service:       service,
		Builder:       m.CommandBuilder,
		ProcessLogDir: m.Config.ProcessLogDir,
	}
	if err := p.Run(); err != nil {
		slog.Error("start managed process", "name", service.Name, "error", err)
		cmd := m.State.ServiceList.NewStatusMessage(err.Error())
		return m, tea.Batch(cmd, clearStatusAfter(2*time.Second))
	}
	m.RunningInstances[service.Name] = p
	m.State.SetActivePanel("running")

	cmd := m.State.RunningList.InsertItem(len(m.State.RunningList.Items()), process.NewItem(p))

	return m, cmd
}

func (m Model) handleAuthModalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		if m.authModal.session != nil {
			_ = m.authModal.session.Interrupt()
		}
		return m, nil
	}

	input := encodeKeyForPTY(msg)
	if len(input) == 0 || m.authModal.session == nil {
		return m, nil
	}

	return m, writeAuthInputCmd(m.authModal.session, input)
}

func encodeKeyForPTY(msg tea.KeyPressMsg) []byte {
	switch msg.String() {
	case "enter":
		return []byte("\r")
	case "tab":
		return []byte("\t")
	case "backspace", "ctrl+h":
		return []byte{0x7f}
	case "esc":
		return []byte{0x1b}
	case "ctrl+d":
		return []byte{0x04}
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	default:
		if t := msg.Text; t != "" {
			return []byte(t)
		}
		return nil
	}
}

func (m *Model) initAuthModalSkeleton(service tui.Service, command string) {
	label := authCommandLabel(command)
	m.authModal = authModalState{
		active:          true,
		commandLabel:    label,
		selectedService: service,
		started:         time.Now(),
		output:          []byte("Running " + label + "...\r\n"),
	}
}

func (m *Model) closeAuthModal() {
	if m.authModal.session != nil {
		_ = m.authModal.session.Close()
	}
	m.authModal = authModalState{}
}

func (m *Model) appendAuthOutput(chunk []byte) {
	if len(chunk) == 0 {
		return
	}

	m.authModal.output = append(m.authModal.output, chunk...)
	if len(m.authModal.output) <= authOutputLimitBytes {
		return
	}
	m.authModal.output = append([]byte(nil), m.authModal.output[len(m.authModal.output)-authOutputLimitBytes:]...)
}

func (m Model) resizeAuthPTYToModal() error {
	if m.authModal.session == nil {
		return nil
	}
	w, h := m.authModalContentSize()
	return m.authModal.session.Resize(w, h)
}

func (m Model) authModalContentSize() (int, int) {
	modalW, modalH := m.authModalOuterSize()
	w := modalW - 6
	h := modalH - 6

	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	return w, h
}

func (m Model) authModalOuterSize() (int, int) {
	w := m.State.UserInterface.Width
	h := m.State.UserInterface.Height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	modalW := w * 2 / 3
	modalH := h * 3 / 5

	modalW = max(modalW, 56)
	modalH = max(modalH, 12)

	modalW = min(modalW, w-4)
	modalH = min(modalH, h-2)

	if modalW < 1 {
		modalW = 1
	}
	if modalH < 1 {
		modalH = 1
	}

	return modalW, modalH
}

func authCommandLabel(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return "auth command"
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "auth command"
	}
	return fields[0]
}

func (m Model) View() tea.View {
	if m.authModal.active {
		return m.authModalView()
	}

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

func (m Model) authModalView() tea.View {
	w := m.State.UserInterface.Width
	h := m.State.UserInterface.Height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	bodyW, bodyH := m.authModalContentSize()
	modalW, modalH := m.authModalOuterSize()
	output := string(m.authModal.output)
	if strings.TrimSpace(output) == "" {
		output = "Waiting for command output..."
	}
	if bodyH > 0 {
		output = trimToLastLines(output, bodyH)
	}

	title := m.authModal.commandLabel + " authentication"
	header := lipgloss.NewStyle().Bold(true).Render(title)
	footer := lipgloss.NewStyle().Faint(true).Render("Enter: submit  Ctrl+C: cancel")
	body := lipgloss.NewStyle().Width(bodyW).Height(bodyH).Align(lipgloss.Left, lipgloss.Top).Render(output)
	cardContent := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	card := tui.PanelStyle(modalW, modalH, true).Render(cardContent)
	view := lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, card)

	v := tea.NewView(view)
	v.AltScreen = true
	return v
}

func trimToLastLines(s string, maxLines int) string {
	if maxLines < 1 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

func (m Model) finishAuthSuccess() (tea.Model, tea.Cmd) {
	srv := m.authModal.selectedService
	m.startInProgress = false
	m.closeAuthModal()

	nextModel, startCmd := m.startService(srv)
	next, ok := nextModel.(Model)
	if !ok {
		return nextModel, startCmd
	}
	if _, ok := next.RunningInstances[srv.Name]; !ok {
		return next, startCmd
	}

	statusCmd := next.State.RunningList.NewStatusMessage("aws-mfa succeeded")
	return next, tea.Batch(startCmd, statusCmd, clearStatusAfter(2*time.Second))
}
