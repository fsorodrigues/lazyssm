//go:build !darwin && !linux && !windows

package process

import (
	"os"
	"os/exec"
)

func configureManagedCommand(cmd *exec.Cmd) {}

func terminateManagedProcess(pid int) error {
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
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		return false, nil
	}

	return true, nil
}

func terminateManagedProcessForce(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	if err := proc.Kill(); err != nil {
		return err
	}

	return nil
}
