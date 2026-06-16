package model

import (
	"bytes"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"lazyssm/process"
	"lazyssm/tui"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	authOutputLimitBytes = 64 * 1024
	authModalMinVisible  = 1200 * time.Millisecond
	shutdownMaxWait      = 5 * time.Second
)

var deleteKey = key.NewBinding(
	key.WithKeys("ctrl+d"),
	key.WithHelp("ctrl+d", "stop"),
)

type (
	refreshMsg            time.Time
	clearStatusMsg        struct{}
	authSessionStartedMsg struct {
		service tui.Service
		session *process.AuthPTYSession
		err     error
	}
	authExecFinishedMsg struct {
		service tui.Service
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
	deleteFinishedMsg     struct {
		name string
		err  error
	}
	shutdownGracefulFinishedMsg struct {
		failed map[string]error
	}
	shutdownDeadlineMsg struct{}
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

func runAuthExecCmd(command string, service tui.Service) tea.Cmd {
	cmd, err := process.BuildAuthPreflightCommand(command)
	if err != nil {
		return func() tea.Msg {
			return authExecFinishedMsg{service: service, err: err}
		}
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return authExecFinishedMsg{service: service, err: err}
	})
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

func authSuccessProceedCmd(d time.Duration) tea.Cmd {
	if d <= 0 {
		return func() tea.Msg { return authSuccessProceedMsg{} }
	}

	return tea.Tick(d, func(time.Time) tea.Msg {
		return authSuccessProceedMsg{}
	})
}

type Model struct {
	Config            tui.Config
	State             tui.State
	RunningInstances  map[string]*process.Proc
	CommandBuilder    process.CommandBuilder
	AuthCommand       string
	SkipAuth          bool
	Simulate          bool
	pendingDelete     bool
	pendingDeleteName string
	startInProgress   bool
	authModal         authModalState
	deleting          map[string]bool
	spinner           spinner.Model
	shuttingDown      bool
	shutdownStarted   time.Time
	shutdownPending   map[string]*process.Proc
	shutdownFailed    map[string]error
}

func InitModel(
	cfg *tui.Config,
	builder process.CommandBuilder,
	authCommand string,
	skipAuth bool,
	simulate bool,
) Model {
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
		deleting:         make(map[string]bool),
		shutdownPending:  make(map[string]*process.Proc),
		shutdownFailed:   make(map[string]error),
		spinner: spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
			spinner.WithStyle(lipgloss.NewStyle()),
		),
	}
}

func (m Model) Init() tea.Cmd {
	return tickRefresh()
}

func (m Model) Cleanup() {
	m.forceCleanup()
}

func startGracefulShutdownCmd(procs map[string]*process.Proc) tea.Cmd {
	return func() tea.Msg {
		failed := make(map[string]error)
		for name, p := range procs {
			if p == nil {
				continue
			}
			exited, err := p.StopGracefully()
			if err != nil {
				failed[name] = err
				continue
			}
			if !exited {
				failed[name] = nil
			}
		}
		return shutdownGracefulFinishedMsg{failed: failed}
	}
}

func shutdownDeadlineCmd() tea.Cmd {
	return tea.Tick(shutdownMaxWait, func(time.Time) tea.Msg {
		return shutdownDeadlineMsg{}
	})
}

func (m *Model) forceCleanup() {
	m.startInProgress = false
	if m.authModal.session != nil {
		_ = m.authModal.session.Interrupt()
	}
	m.closeAuthModal()

	for name, p := range m.RunningInstances {
		if m.deleting[name] {
			slog.Info("skipping cleanup for process being stopped async", "name", name)
			continue
		}
		snapshot := p.Snapshot()
		slog.Info(
			"cleaning up process",
			"name",
			name,
			"pid",
			snapshot.PID,
			"status",
			snapshot.Status,
		)
		if err := p.ForceKill(); err != nil {
			slog.Error("cleanup: failed to kill process", "name", name, "error", err)
		}
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
			if process.IsAuthPTYUnsupported(msg.err) {
				m.closeAuthModal()
				return m, runAuthExecCmd(m.AuthCommand, msg.service)
			}

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

	case authExecFinishedMsg:
		m.startInProgress = false
		if msg.err != nil {
			slog.Warn("aws-mfa preflight failed", "error", msg.err)
			cmd := m.State.ServiceList.NewStatusMessage("aws-mfa failed")
			return m, tea.Batch(cmd, clearStatusAfter(2*time.Second))
		}

		return m.startService(msg.service)

	case authSuccessProceedMsg:
		if !m.authModal.active {
			return m, nil
		}
		return m.finishAuthSuccess()

	case refreshMsg:
		m.refreshRunningItems()
		return m, tickRefresh()

	case spinner.TickMsg:
		var spinCmd tea.Cmd
		m.spinner, spinCmd = m.spinner.Update(msg)
		m.refreshRunningItems()
		if len(m.deleting) > 0 {
			return m, spinCmd
		}
		return m, nil

	case deleteFinishedMsg:
		name := msg.name
		delete(m.deleting, name)
		if msg.err != nil {
			slog.Error("failed to stop process", "name", name, "error", msg.err)
			statusCmd := m.State.RunningList.NewStatusMessage("failed to stop " + name)
			return m, tea.Batch(statusCmd, clearStatusAfter(2*time.Second))
		}
		delete(m.RunningInstances, name)
		items := m.State.RunningList.Items()
		for i, item := range items {
			if item.FilterValue() == name {
				m.State.RunningList.RemoveItem(i)
				m.State.RunningList.Select(max(i-1, 0))
				break
			}
		}
		if len(m.RunningInstances) == 0 {
			m.State.SetActivePanel("services")
		}
		statusCmd := m.State.RunningList.NewStatusMessage("stopped " + name)
		return m, tea.Batch(statusCmd, clearStatusAfter(2*time.Second))

	case shutdownGracefulFinishedMsg:
		m.shutdownFailed = msg.failed
		if len(msg.failed) == 0 {
			return m, tea.Quit
		}
		return m, nil

	case shutdownDeadlineMsg:
		return m, tea.Quit

	case tea.KeyPressMsg:
		if m.authModal.active {
			return m.handleAuthModalKey(msg)
		}
		if m.shuttingDown {
			return m, nil
		}

		switch {
		case msg.String() == "ctrl+c" || (msg.String() == "q" && !m.pendingDelete):
			return m.beginShutdown()
		case msg.String() == "ctrl+z":
			return m, tea.Suspend
		case msg.String() == "tab":
			if m.pendingDelete {
				break
			}
			m.State.CircleActivePanel()
		case key.Matches(msg, deleteKey):
			if m.State.ActivePanel == "running" && len(m.State.RunningList.Items()) > 0 {
				selection := m.State.RunningList.SelectedItem()
				if selection == nil {
					break
				}

				name := selection.FilterValue()
				m.pendingDelete = true
				m.pendingDeleteName = name
				m.refreshRunningItems()
				m.State.RunningList.StatusMessageLifetime = time.Hour
				cmd := m.State.RunningList.NewStatusMessage(
					"stop " + name + "? enter confirm, esc cancel",
				)
				return m, cmd
			}
		case msg.String() == "escape" || msg.String() == "esc":
			if m.pendingDelete {
				m.pendingDelete = false
				m.pendingDeleteName = ""
				m.refreshRunningItems()
				m.State.RunningList.StatusMessageLifetime = time.Second
				cmd := m.State.RunningList.NewStatusMessage("")
				return m, cmd
			}
		case msg.String() == "enter":
			if m.pendingDelete && m.State.ActivePanel == "running" {
				name := m.pendingDeleteName
				m.pendingDelete = false
				m.pendingDeleteName = ""

				if name == "" {
					selection := m.State.RunningList.SelectedItem()
					if selection != nil {
						name = selection.FilterValue()
					}
				}

				if name == "" {
					break
				}

				if m.deleting[name] {
					break
				}

				if p, ok := m.RunningInstances[name]; ok {
					snapshot := p.Snapshot()
					slog.Info(
						"async stop started",
						"name",
						name,
						"pid",
						snapshot.PID,
						"status",
						snapshot.Status,
					)
					m.deleting[name] = true
					m.refreshRunningItems()
					startedSpinner := len(m.deleting) == 1
					statusCmd := m.State.RunningList.NewStatusMessage("stopping " + name + "...")
					killCmd := func() tea.Msg {
						err := p.Kill()
						return deleteFinishedMsg{name: name, err: err}
					}
					cmds := []tea.Cmd{statusCmd, killCmd}
					if startedSpinner {
						cmds = append(cmds, m.spinner.Tick)
					}
					return m, tea.Batch(cmds...)
				}

				m.refreshRunningItems()
				statusCmd := m.State.RunningList.NewStatusMessage(name + " is no longer running")
				return m, tea.Batch(statusCmd, clearStatusAfter(2*time.Second))
			}
			if m.State.ActivePanel == "services" {
				// While the user is actively typing a filter query, Enter should
				// confirm/close the filter — not start a service. Let the event
				// fall through to ServiceList.Update so the list handles it.
				if m.State.ServiceList.FilterState() == list.Filtering {
					break
				}

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

func (m Model) beginShutdown() (tea.Model, tea.Cmd) {
	m.shuttingDown = true
	m.shutdownStarted = time.Now()
	m.shutdownPending = make(map[string]*process.Proc)
	m.shutdownFailed = make(map[string]error)
	m.pendingDelete = false
	m.pendingDeleteName = ""
	m.startInProgress = false

	if m.authModal.session != nil {
		_ = m.authModal.session.Interrupt()
	}
	m.closeAuthModal()

	for name, p := range m.RunningInstances {
		if m.deleting[name] || p == nil {
			continue
		}
		m.shutdownPending[name] = p
	}

	if len(m.shutdownPending) == 0 {
		return m, tea.Quit
	}

	return m, tea.Batch(startGracefulShutdownCmd(m.shutdownPending), shutdownDeadlineCmd())
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
		item := process.NewItem(p)
		if m.deleting[name] {
			item.Deleting = true
			item.Frame = m.spinner.View()
		} else if m.pendingDelete && m.pendingDeleteName == name {
			item.PendingDelete = true
		}
		items = append(items, item)
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

	input := encodeKeyForPTY(msg, m.authModal.session)
	if len(input) == 0 || m.authModal.session == nil {
		return m, nil
	}

	if err := m.authModal.session.WriteInput(input); err != nil {
		slog.Debug("write auth PTY input", "error", err)
	}

	return m, nil
}

func encodeKeyForPTY(msg tea.KeyPressMsg, session *process.AuthPTYSession) []byte {
	switch msg.String() {
	case "enter":
		return []byte("\r")
	case "tab":
		return []byte("\t")
	case "backspace", "backspace2", "ctrl+?", "\x7f":
		if session == nil {
			return []byte{0x7f}
		}
		return []byte{session.EraseByte()}
	case "ctrl+h", "\b":
		return []byte{0x08}
	case "delete":
		return []byte("\x1b[3~")
	case "esc", "escape":
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

	m.authModal.output = applyAuthOutputChunk(m.authModal.output, chunk)
	if len(m.authModal.output) <= authOutputLimitBytes {
		return
	}
	m.authModal.output = append(
		[]byte(nil),
		m.authModal.output[len(m.authModal.output)-authOutputLimitBytes:]...)
}

func applyAuthOutputChunk(dst []byte, chunk []byte) []byte {
	for i := 0; i < len(chunk); i++ {
		switch chunk[i] {
		case '\r':
			if i+1 < len(chunk) && chunk[i+1] == '\n' {
				dst = append(dst, '\n')
				i++
				continue
			}
			lineStart := bytes.LastIndexByte(dst, '\n')
			if lineStart == -1 {
				dst = dst[:0]
			} else {
				dst = dst[:lineStart+1]
			}
		case '\b', 0x7f:
			dst = trimLastDisplayRune(dst)
		case 0x1b:
			n := ansiSequenceLen(chunk[i:])
			if n > 0 {
				i += n - 1
			}
		case '\n', '\t':
			dst = append(dst, chunk[i])
		default:
			if chunk[i] < 0x20 {
				continue
			}
			dst = append(dst, chunk[i])
		}
	}

	return dst
}

func trimLastDisplayRune(dst []byte) []byte {
	if len(dst) == 0 || dst[len(dst)-1] == '\n' {
		return dst
	}

	_, size := utf8.DecodeLastRune(dst)
	if size <= 0 {
		return dst[:len(dst)-1]
	}

	return dst[:len(dst)-size]
}

func ansiSequenceLen(chunk []byte) int {
	if len(chunk) < 2 || chunk[0] != 0x1b {
		return 0
	}

	if chunk[1] != '[' && chunk[1] != ']' && chunk[1] != '(' && chunk[1] != ')' && chunk[1] != 'O' {
		return 2
	}

	for i := 2; i < len(chunk); i++ {
		if chunk[i] >= 0x40 && chunk[i] <= 0x7e {
			return i + 1
		}
	}

	return len(chunk)
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
	if m.shuttingDown {
		return m.shutdownView()
	}
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

func (m Model) shutdownView() tea.View {
	w := m.State.UserInterface.Width
	h := m.State.UserInterface.Height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	bodyW := min(max(w*2/3, 40), max(w-4, 1))
	bodyH := min(max(h/3, 8), max(h-2, 1))

	message := "Winding down processes. Waiting for graceful shutdown..."
	if len(m.shutdownFailed) > 0 {
		message = "Winding down processes. Force cleanup will continue when the 5s limit is reached."
	}

	remaining := shutdownMaxWait - time.Since(m.shutdownStarted)
	if remaining < 0 {
		remaining = 0
	}

	lines := []string{
		lipgloss.NewStyle().Bold(true).Render("Shutting down lazyssm"),
		"",
		message,
		"",
		"Time remaining: " + remaining.Round(time.Second).String(),
	}
	if count := len(m.shutdownPending); count > 0 {
		lines = append(lines, "Managed processes: "+strconv.Itoa(count))
	}
	if count := len(m.shutdownFailed); count > 0 {
		lines = append(lines, "Still running: "+strconv.Itoa(count))
	}

	body := lipgloss.NewStyle().
		Width(max(bodyW-4, 1)).
		Height(max(bodyH-4, 1)).
		Align(lipgloss.Left, lipgloss.Top).
		Render(strings.Join(lines, "\n"))
	card := tui.PanelStyle(bodyW, bodyH, true).Render(body)
	view := lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, card)

	v := tea.NewView(view)
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
	body := lipgloss.NewStyle().
		Width(bodyW).
		Height(bodyH).
		Align(lipgloss.Left, lipgloss.Top).
		Render(output)
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
