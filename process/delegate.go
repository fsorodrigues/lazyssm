package process

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("228"))
	itemStyle   = lipgloss.NewStyle().PaddingLeft(2)
	outputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			PaddingLeft(4)
	selectedStyle       = lipgloss.NewStyle().PaddingLeft(1).Inherit(borderStyle)
	selectedOutputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				PaddingLeft(3).
				Inherit(borderStyle)
)

// RunningDelegate is a custom ItemDelegate for the running services list.
// It renders each item as two lines: name/PID/status, and last process output.
type RunningDelegate struct {
	ShortHelpFunc func() []key.Binding
	FullHelpFunc  func() [][]key.Binding
}

func (d RunningDelegate) Height() int  { return 2 }
func (d RunningDelegate) Spacing() int { return 1 }

func (d RunningDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d RunningDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	procItem, ok := item.(Item)
	if !ok {
		return
	}

	proc := procItem.Process
	pid := 0
	status := "unknown"
	output := ""
	if proc != nil {
		snapshot := proc.Snapshot()
		pid = snapshot.PID
		status = snapshot.Status
		output = snapshot.LastLine
	}

	// Line 1: name | PID | status
	line1 := fmt.Sprintf("%s  pid:%d  [%s]", procItem.Title(), pid, status)

	// Line 2: last output line (truncated to available width)
	maxW := m.Width() - 6
	if maxW > 0 && len(output) > maxW {
		output = output[:maxW-1] + "…"
	}
	if output == "" {
		output = "(no output)"
	}

	if index == m.Index() {
		line1 = selectedStyle.Render(line1)
		line2 := selectedOutputStyle.Render(output)
		fmt.Fprintf(w, "%s\n%s", line1, line2)
	} else {
		line1 = itemStyle.Render(line1)
		line2 := outputStyle.Render(output)
		fmt.Fprintf(w, "%s\n%s", line1, line2)
	}
}

func (d RunningDelegate) ShortHelp() []key.Binding {
	if d.ShortHelpFunc != nil {
		return d.ShortHelpFunc()
	}
	return nil
}

func (d RunningDelegate) FullHelp() [][]key.Binding {
	if d.FullHelpFunc != nil {
		return d.FullHelpFunc()
	}
	return nil
}
