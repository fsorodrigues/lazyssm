//go:build windows

package process

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const gracefulShutdownTimeout = 2 * time.Second

func configureManagedCommand(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NEW_PROCESS_GROUP
}

func terminateManagedProcess(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid process id %d", pid)
	}

	exited, err := terminateManagedProcessGracefully(pid)
	if err != nil {
		return err
	}
	if exited {
		return nil
	}

	return terminateManagedProcessForce(pid)
}

func terminateManagedProcessGracefully(pid int) (bool, error) {
	if pid <= 0 {
		return false, fmt.Errorf("invalid process id %d", pid)
	}

	if err := taskkillProcessTree(pid, false); err != nil {
		slog.Debug("graceful taskkill attempted", "pid", pid, "error", err)
	}

	exited, err := waitForProcessExit(pid, gracefulShutdownTimeout)
	if err != nil {
		return false, fmt.Errorf("wait for graceful shutdown of process %d: %w", pid, err)
	}

	return exited, nil

}

func terminateManagedProcessForce(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid process id %d", pid)
	}

	if err := taskkillProcessTree(pid, true); err != nil {
		return fmt.Errorf("forced taskkill failed for process tree %d: %w", pid, err)
	}

	exited, err := waitForProcessExit(pid, gracefulShutdownTimeout)
	if err != nil {
		return fmt.Errorf("wait for forced shutdown of process %d: %w", pid, err)
	}
	if exited {
		return nil
	}

	return fmt.Errorf("process tree %d did not exit after forced taskkill", pid)
}

func taskkillProcessTree(pid int, force bool) error {
	args := []string{"/PID", strconv.Itoa(pid), "/T"}
	if force {
		args = append(args, "/F")
	}

	output, err := exec.Command("taskkill", args...).CombinedOutput()
	if err == nil {
		return nil
	}

	exited, waitErr := waitForProcessExit(pid, 0)
	if waitErr == nil && exited {
		return nil
	}
	if waitErr != nil {
		return fmt.Errorf("taskkill error: %w (wait check failed: %v)", err, waitErr)
	}

	return fmt.Errorf("taskkill error: %w: %s", err, output)
}

func waitForProcessExit(pid int, timeout time.Duration) (bool, error) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return true, nil
		}
		return false, err
	}
	defer windows.CloseHandle(handle)

	timeoutMs := durationToTimeoutMs(timeout)
	status, err := windows.WaitForSingleObject(handle, timeoutMs)
	if err != nil {
		return false, err
	}

	switch status {
	case windows.WAIT_OBJECT_0:
		return true, nil
	case uint32(windows.WAIT_TIMEOUT):
		return false, nil
	default:
		return false, fmt.Errorf("unexpected wait status %d", status)
	}
}

func durationToTimeoutMs(timeout time.Duration) uint32 {
	if timeout <= 0 {
		return 0
	}

	ms := timeout / time.Millisecond
	if ms <= 0 {
		return 1
	}
	if ms > time.Duration(^uint32(0)) {
		return windows.INFINITE
	}

	return uint32(ms)
}
