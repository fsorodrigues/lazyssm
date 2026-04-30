package process

import (
	"fmt"
	"os/exec"
	"strings"
)

func BuildAuthPreflightCommand(command string) (*exec.Cmd, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return nil, fmt.Errorf("auth command is empty")
	}

	return exec.Command(trimmed), nil
}
