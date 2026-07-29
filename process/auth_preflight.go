package process

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mattn/go-shellwords"
)

func ParseAuthCommand(command string) ([]string, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return nil, fmt.Errorf("auth command is empty")
	}

	args, err := shellwords.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("auth command is empty")
	}

	return args, nil
}

func BuildAuthCommand(command string) (*exec.Cmd, error) {
	args, err := ParseAuthCommand(command)
	if err != nil {
		return nil, err
	}

	return exec.Command(args[0], args[1:]...), nil
}
