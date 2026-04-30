//go:build !darwin && !linux && !windows

package process

import (
	"os"
	"os/exec"
)

func configureManagedCommand(cmd *exec.Cmd) {}

func terminateManagedProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		if err := proc.Kill(); err != nil {
			return err
		}
	}

	return nil
}
