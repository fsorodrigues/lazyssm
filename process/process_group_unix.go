//go:build darwin || linux

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

const (
	gracefulShutdownTimeout      = 2 * time.Second
	gracefulShutdownPollInterval = 100 * time.Millisecond
)

func configureManagedCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateManagedProcess(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid process id %d", pid)
	}

	if err := signalProcessGroup(pid, syscall.SIGINT); err != nil {
		return err
	}
	if waitForProcessGroupExit(pid, gracefulShutdownTimeout) {
		return nil
	}

	if err := signalProcessGroup(pid, syscall.SIGKILL); err != nil {
		return err
	}
	if waitForProcessGroupExit(pid, gracefulShutdownTimeout) {
		return nil
	}

	return fmt.Errorf("process group %d did not exit after SIGKILL", pid)
}

func signalProcessGroup(pid int, signal syscall.Signal) error {
	err := syscall.Kill(-pid, signal)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func waitForProcessGroupExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(-pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return true
		}
		if err != nil {
			return false
		}

		time.Sleep(gracefulShutdownPollInterval)
	}

	return false
}
