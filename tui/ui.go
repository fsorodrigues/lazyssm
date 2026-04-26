package tui

import (
	"charm.land/lipgloss/v2"
)

type UserInterface struct {
	Width  int
	Height int
	PanelW int
	PanelH int
	ListH  int
}

func (ui *UserInterface) Resize(w, h int) {
	ui.Width = w
	ui.Height = h
	ui.PanelW = (w - 4) / 2  // subtract 4 for left+right borders on 2 panels
	ui.PanelH = h - 1        // subtract 1 for top row, 2 for top+bottom border
	ui.ListH = ui.PanelH - 2 // list renders 2 extra lines internally
}

func PanelStyle(w, h int, focused bool) lipgloss.Style {
	base := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#3C3C3C")).
		Width(w).
		Height(h).
		Align(lipgloss.Left, lipgloss.Center)

	if focused {
		return base.Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("228"))
	}
	return base
}

func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		MarginLeft(3).
		PaddingLeft(1).
		PaddingRight(1).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4"))
}
