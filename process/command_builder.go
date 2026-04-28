package process

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"

	"lazyssm/tui"
)

func NewCommandBuilder(simulate bool) (CommandBuilder, error) {
	if simulate {
		scriptPath, err := filepath.Abs("scripts/simulate_service.sh")
		if err != nil {
			return nil, err
		}
		return SimulateCommandBuilder{ScriptPath: scriptPath, Interval: 5}, nil
	}

	return SSMCommandBuilder{}, nil
}

type CommandBuilder interface {
	Build(service tui.Service) (*exec.Cmd, error)
}

type SimulateCommandBuilder struct {
	ScriptPath string
	Interval   int
}

func (b SimulateCommandBuilder) Build(service tui.Service) (*exec.Cmd, error) {
	if b.ScriptPath == "" {
		return nil, fmt.Errorf("simulate script path is required")
	}

	interval := b.Interval
	if interval <= 0 {
		interval = 5
	}

	return exec.Command(b.ScriptPath, service.Name, strconv.Itoa(interval)), nil
}

type SSMCommandBuilder struct{}

func (b SSMCommandBuilder) Build(service tui.Service) (*exec.Cmd, error) {
	if service.Target == "" {
		return nil, fmt.Errorf("service %q is missing target", service.Name)
	}
	if service.Profile == "" {
		return nil, fmt.Errorf("service %q is missing profile", service.Name)
	}
	if service.Region == "" {
		return nil, fmt.Errorf("service %q is missing region", service.Name)
	}
	if service.Ports.Port <= 0 {
		return nil, fmt.Errorf("service %q has invalid remote port", service.Name)
	}
	if service.Ports.LocalPort <= 0 {
		return nil, fmt.Errorf("service %q has invalid local port", service.Name)
	}

	parameters := fmt.Sprintf("portNumber=%d,localPortNumber=%d", service.Ports.Port, service.Ports.LocalPort)

	cmd := exec.Command(
		"aws",
		"ssm",
		"start-session",
		"--target", service.Target,
		"--document-name", "AWS-StartPortForwardingSession",
		"--parameters", parameters,
		"--profile", service.Profile,
		"--region", service.Region,
	)

	return cmd, nil
}
