package tui

import (
	"charm.land/lipgloss/v2"
)

var (
	Box     = lipgloss.NewStyle().Width(40).Height(40).Align(lipgloss.Center, lipgloss.Center)
	Border  = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Inherit(Box)
	Focused = lipgloss.NewStyle().Background(lipgloss.Color("2")).Inherit(Border)
)
