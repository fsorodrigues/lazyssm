package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	serviceBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("228"))
	serviceItemStyle = lipgloss.NewStyle().PaddingLeft(2)
	serviceDescStyle = lipgloss.NewStyle().
				PaddingLeft(4).
				Foreground(lipgloss.Color("#888888"))
	serviceSelectedStyle     = lipgloss.NewStyle().PaddingLeft(1).Inherit(serviceBorderStyle)
	serviceSelectedDescStyle = lipgloss.NewStyle().
					PaddingLeft(1).
					Foreground(lipgloss.Color("#888888")).
					Inherit(serviceBorderStyle)

	prodTypeStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true).
			Background(lipgloss.Lighten(lipgloss.Color("202"), 0.3)).
			Foreground(lipgloss.Color("#111827"))
	testTypeStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true).
			Background(lipgloss.Lighten(lipgloss.Color("#0000FF"), 0.25)).
			Foreground(lipgloss.Color("#F8FAFC"))
	unknownTypeStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Bold(true).
				Background(lipgloss.Color("#4B5563")).
				Foreground(lipgloss.Color("#F8FAFC"))
)

type ServiceDelegate struct{}

func (d ServiceDelegate) Height() int  { return 2 }
func (d ServiceDelegate) Spacing() int { return 1 }

func (d ServiceDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d ServiceDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	serviceItem, ok := item.(Item)
	if !ok {
		return
	}

	typeLabel, typeStyle := classifyServiceType(serviceItem)
	typeTag := typeStyle.Render(typeLabel)
	line1 := fmt.Sprintf("%s %s", serviceItem.Title(), typeTag)
	line2 := serviceItem.Description()

	if index == m.Index() {
		fmt.Fprintf(w, "%s\n%s", serviceSelectedStyle.Render(line1), serviceSelectedDescStyle.Render("→ "+line2))
		return
	}

	fmt.Fprintf(w, "%s\n%s", serviceItemStyle.Render(line1), serviceDescStyle.Render(line2))
}

func classifyServiceType(item Item) (string, lipgloss.Style) {
	if item.Service == nil {
		return "UNKNOWN", unknownTypeStyle
	}

	serviceType := strings.ToLower(strings.TrimSpace(item.Service.Type))
	switch serviceType {
	case "prod":
		return "PROD", prodTypeStyle
	case "test":
		return "TEST", testTypeStyle
	default:
		return "UNKNOWN", unknownTypeStyle
	}
}
