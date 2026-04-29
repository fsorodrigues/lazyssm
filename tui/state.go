package tui

import (
	"fmt"

	"charm.land/bubbles/v2/list"
)

type State struct {
	ServiceItems  []list.Item
	RunningItems  []list.Item
	ServiceList   list.Model
	RunningList   list.Model
	ActivePanel   string
	UserInterface UserInterface
}

func NewState() State {
	return State{
		ServiceItems: make([]list.Item, 0),
		RunningItems: make([]list.Item, 0),
		ActivePanel:  "services",
		UserInterface: UserInterface{
			Width:  0,
			Height: 0,
		},
	}
}

func (s *State) SetActivePanel(panel string) {
	s.ActivePanel = panel
}

func (s *State) CircleActivePanel() {
	switch s.ActivePanel {
	case "services":
		s.SetActivePanel("running")
	case "running":
		s.SetActivePanel("services")
	}
}

type Item struct {
	title       string
	description string
	Service     *Service
}

func NewItem(srv *Service) Item {
	return Item{
		title:       srv.Name,
		description: fmt.Sprintf("[%s] %s", srv.Region, srv.Description),
		Service:     srv,
	}
}

func (i Item) FilterValue() string {
	return i.title
}

func (i Item) Title() string {
	return i.title
}

func (i Item) Description() string {
	return i.description
}

type DelegateItem struct {
	Item
}
