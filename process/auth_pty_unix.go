//go:build unix

package process

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func startAuthPTYCommand(command string) (*exec.Cmd, *os.File, byte, error) {
	cmd, err := BuildAuthCommand(command)
	if err != nil {
		return nil, nil, 0, err
	}

	ptmx, tty, err := pty.Open()
	if err != nil {
		return nil, nil, 0, err
	}

	defer func() {
		if err != nil {
			_ = ptmx.Close()
			_ = tty.Close()
		}
	}()

	if termios, termErr := unix.IoctlGetTermios(int(tty.Fd()), unix.TCGETS); termErr == nil {
		termios.Cc[unix.VERASE] = 0x7f
		_ = unix.IoctlSetTermios(int(tty.Fd()), unix.TCSETS, termios)
	}

	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}

	if err = cmd.Start(); err != nil {
		return nil, nil, 0, err
	}

	_ = tty.Close()

	return cmd, ptmx, 0x7f, nil
}
