//go:build unix

package process

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func startAuthPTYCommand(command string) (*exec.Cmd, *os.File, byte, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil, nil, 0, fmt.Errorf("auth command is empty")
	}

	cmd := exec.Command(fields[0], fields[1:]...)

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
