//go:build !unix

package process

import (
	"fmt"
	"os"
	"os/exec"
)

func startAuthPTYCommand(command string) (*exec.Cmd, *os.File, byte, error) {
	return nil, nil, 0, fmt.Errorf("auth PTY mode is unsupported on this platform")
}
