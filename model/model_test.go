package model

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"lazyssm/process"
	"lazyssm/tui"
)

func TestEscapeCancelsPendingDelete(t *testing.T) {
	m := InitModel(&tui.Config{}, nil, "", false, false)
	m.State.SetActivePanel("running")

	proc := &process.Proc{
		Name:   "svc",
		PID:    123,
		Status: process.StatusRunning,
	}
	m.RunningInstances[proc.Name] = proc
	m.State.RunningItems = []list.Item{process.NewItem(proc)}
	m.State.RunningList.SetItems(m.State.RunningItems)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyExtended, Text: "ctrl+d"})
	withPendingDelete, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected model.Model after ctrl+d update, got %T", updated)
	}

	if !withPendingDelete.pendingDelete {
		t.Fatal("expected pendingDelete to be true after ctrl+d")
	}
	if got := withPendingDelete.State.RunningList.StatusMessageLifetime; got != time.Hour {
		t.Fatalf("expected running list status lifetime to be %s after ctrl+d, got %s", time.Hour, got)
	}

	updated, _ = withPendingDelete.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	withoutPendingDelete, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected model.Model after esc update, got %T", updated)
	}

	if withoutPendingDelete.pendingDelete {
		t.Fatal("expected pendingDelete to be false after esc")
	}
	if got := withoutPendingDelete.State.RunningList.StatusMessageLifetime; got != time.Second {
		t.Fatalf("expected running list status lifetime to be %s after esc, got %s", time.Second, got)
	}
	if got := withoutPendingDelete.State.ActivePanel; got != "running" {
		t.Fatalf("expected active panel to remain running after esc, got %q", got)
	}
}
