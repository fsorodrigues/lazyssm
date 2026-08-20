package model

import (
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"lazyssm/process"
	"lazyssm/tui"
)

func newRunningModelWithService(t *testing.T) Model {
	t.Helper()

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

	return m
}

func newAuthModel(t *testing.T) Model {
	t.Helper()
	m := InitModel(&tui.Config{}, nil, "aws-mfa", false, false)
	m.initAuthModalSkeleton(tui.Service{Name: "svc"}, m.AuthCommand)
	return m
}

func TestAuthFailureKeepsOutputVisible(t *testing.T) {
	m := newAuthModel(t)

	updated, _ := m.Update(authOutputMsg{chunk: []byte("login failed\r\n"), ok: true})
	m = updated.(Model)
	updated, _ = m.Update(authOutputMsg{ok: false})
	m = updated.(Model)
	updated, _ = m.Update(authExitMsg{err: errors.New("exit status 1")})
	m = updated.(Model)

	if !m.authModal.active || !m.authModal.failed {
		t.Fatal("expected failed auth modal to remain active")
	}
	if !strings.Contains(string(m.authModal.output), "login failed") {
		t.Fatalf("expected auth output to be retained, got %q", m.authModal.output)
	}
	if !strings.Contains(string(m.authModal.output), "exit status 1") {
		t.Fatalf("expected exit error in auth output, got %q", m.authModal.output)
	}
}

func TestAuthFailureCanBeDismissed(t *testing.T) {
	m := newAuthModel(t)
	m.authModal.failed = true

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)
	if m.authModal.active {
		t.Fatal("expected failed auth modal to close on escape")
	}
}

func TestAuthFailureCanBeRetried(t *testing.T) {
	m := newAuthModel(t)
	m.authModal.failed = true

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if !m.authModal.active || m.authModal.failed {
		t.Fatal("expected retry to reset failed auth modal")
	}
	if !m.startInProgress || cmd == nil {
		t.Fatal("expected retry to start auth session")
	}
}

func TestAuthFailureAutoClosesAfterTimeout(t *testing.T) {
	m := newAuthModel(t)
	updated, _ := m.Update(authOutputMsg{ok: false})
	m = updated.(Model)
	updated, cmd := m.Update(authExitMsg{err: errors.New("exit status 1")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected auth failure timeout command")
	}

	updated, _ = m.Update(authFailureTimeoutMsg{failureID: m.authModal.failureID})
	m = updated.(Model)
	if m.authModal.active {
		t.Fatal("expected auth failure modal to auto-close")
	}
}

func TestStaleAuthFailureTimeoutDoesNotCloseRetry(t *testing.T) {
	m := newAuthModel(t)
	updated, _ := m.Update(authOutputMsg{ok: false})
	m = updated.(Model)
	updated, _ = m.Update(authExitMsg{err: errors.New("first failure")})
	m = updated.(Model)
	oldFailureID := m.authModal.failureID

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	updated, _ = m.Update(authOutputMsg{ok: false})
	m = updated.(Model)
	updated, _ = m.Update(authExitMsg{err: errors.New("second failure")})
	m = updated.(Model)

	updated, _ = m.Update(authFailureTimeoutMsg{failureID: oldFailureID})
	m = updated.(Model)
	if !m.authModal.active || !m.authModal.failed {
		t.Fatal("expected stale timeout not to close current failure modal")
	}
}

func TestAuthFailureCountdownRoundsUp(t *testing.T) {
	failedAt := time.Date(2026, time.August, 21, 12, 0, 0, 0, time.UTC)

	if got := authFailureRemaining(failedAt.Add(500*time.Millisecond), failedAt); got != 14*time.Second+500*time.Millisecond {
		t.Fatalf("remaining duration = %s, want 14.5s", got)
	}
	if got := (int((authFailureRemaining(failedAt.Add(500*time.Millisecond), failedAt) + time.Second - 1) / time.Second)); got != 15 {
		t.Fatalf("remaining seconds = %d, want 15", got)
	}
}

func TestExecAuthFailureDoesNotOpenFailureModal(t *testing.T) {
	m := newAuthModel(t)
	m.closeAuthModal()

	updated, cmd := m.Update(authExecFinishedMsg{
		service: tui.Service{Name: "svc"},
		err:     errors.New("exit status 1"),
	})
	m = updated.(Model)
	if m.authModal.active {
		t.Fatal("expected exec fallback failure not to open auth modal")
	}
	if m.startInProgress {
		t.Fatal("expected auth attempt to finish")
	}
	if cmd == nil {
		t.Fatal("expected failure status command")
	}
}

func requireRunningItemPendingDelete(t *testing.T, m Model, want bool) {
	t.Helper()

	items := m.State.RunningList.Items()
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 running item, got %d", len(items))
	}

	item, ok := items[0].(process.Item)
	if !ok {
		t.Fatalf("expected process.Item in running list, got %T", items[0])
	}

	if item.PendingDelete != want {
		t.Fatalf("expected running item PendingDelete=%t, got %t", want, item.PendingDelete)
	}
}

func TestEscapeCancelsPendingDelete(t *testing.T) {
	m := newRunningModelWithService(t)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyExtended, Text: "ctrl+d"})
	withPendingDelete, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected model.Model after ctrl+d update, got %T", updated)
	}

	if !withPendingDelete.pendingDelete {
		t.Fatal("expected pendingDelete to be true after ctrl+d")
	}
	requireRunningItemPendingDelete(t, withPendingDelete, true)
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
	requireRunningItemPendingDelete(t, withoutPendingDelete, false)
	if got := withoutPendingDelete.State.RunningList.StatusMessageLifetime; got != time.Second {
		t.Fatalf("expected running list status lifetime to be %s after esc, got %s", time.Second, got)
	}
	if got := withoutPendingDelete.State.ActivePanel; got != "running" {
		t.Fatalf("expected active panel to remain running after esc, got %q", got)
	}
}

func TestQuitWorksWhileDeleteIsPending(t *testing.T) {
	m := newRunningModelWithService(t)
	m.pendingDelete = true
	m.pendingDeleteName = "svc"
	m.refreshRunningItems()
	m.State.RunningList.StatusMessageLifetime = time.Hour

	updated, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	updatedModel, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected model.Model after q update, got %T", updated)
	}

	if !updatedModel.shuttingDown {
		t.Fatal("expected shutdown to start after q during pending delete")
	}
	if updatedModel.pendingDelete {
		t.Fatal("expected pendingDelete to be cleared when quitting")
	}
	if got := updatedModel.State.RunningList.StatusMessageLifetime; got != time.Second {
		t.Fatalf("expected running list status lifetime to reset to %s when quitting, got %s", time.Second, got)
	}
	if cmd == nil {
		t.Fatal("expected shutdown command after q during pending delete")
	}
	if got := cmd(); got == nil {
		t.Fatal("expected shutdown batch command to produce a message")
	}
}

func TestTabCancelsPendingDeleteAndSwitchesPanel(t *testing.T) {
	m := newRunningModelWithService(t)
	m.pendingDelete = true
	m.pendingDeleteName = "svc"
	m.refreshRunningItems()
	m.State.RunningList.StatusMessageLifetime = time.Hour

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	updatedModel, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected model.Model after tab update, got %T", updated)
	}

	if updatedModel.pendingDelete {
		t.Fatal("expected pendingDelete to be cleared after tab")
	}
	requireRunningItemPendingDelete(t, updatedModel, false)
	if got := updatedModel.State.ActivePanel; got != "services" {
		t.Fatalf("expected active panel to switch to services after tab, got %q", got)
	}
	if got := updatedModel.State.RunningList.StatusMessageLifetime; got != time.Second {
		t.Fatalf("expected running list status lifetime to reset to %s after tab, got %s", time.Second, got)
	}
}

func TestNonEscapeKeyCancelsPendingDeleteImmediately(t *testing.T) {
	m := newRunningModelWithService(t)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyExtended, Text: "ctrl+d"})
	withPendingDelete, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected model.Model after ctrl+d update, got %T", updated)
	}

	requireRunningItemPendingDelete(t, withPendingDelete, true)

	updated, _ = withPendingDelete.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	withoutPendingDelete, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected model.Model after right update, got %T", updated)
	}

	if withoutPendingDelete.pendingDelete {
		t.Fatal("expected pendingDelete to be false after non-escape key")
	}
	requireRunningItemPendingDelete(t, withoutPendingDelete, false)
}
