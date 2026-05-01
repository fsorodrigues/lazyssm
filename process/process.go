package process

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"lazyssm/tui"
)

const (
	StatusRunning  = "running"
	StatusExited   = "exited"
	StatusFailed   = "failed"
	StatusKilled   = "killed"
	StatusDeleting = "deleting"
)

type Proc struct {
	Name          string
	Cmd           string
	PID           int
	Outfile       string
	Status        string
	Process       *exec.Cmd
	LogFile       *os.File
	LastLine      string
	ProcessLogDir string
	ExitCode      int
	Service       tui.Service
	Builder       CommandBuilder

	mu sync.RWMutex
}

type Snapshot struct {
	Name     string
	PID      int
	Status   string
	LastLine string
	Outfile  string
	ExitCode int
}

func (p *Proc) Run() error {
	logFileName := fmt.Sprintf("process-%s.log", p.Name)
	logFilePath := filepath.Join(p.ProcessLogDir, logFileName)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}

	if p.Builder == nil {
		logFile.Close()
		return fmt.Errorf("no command builder configured for %q", p.Name)
	}

	cmd, err := p.Builder.Build(p.Service)
	if err != nil {
		logFile.Close()
		return err
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile
	p.Cmd = cmd.String()
	configureManagedCommand(cmd)

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}

	p.mu.Lock()
	p.Process = cmd
	p.PID = cmd.Process.Pid
	p.Status = StatusRunning
	p.LogFile = logFile
	p.Outfile = logFilePath
	p.ExitCode = 0
	p.mu.Unlock()

	slog.Info("background process started", "name", p.Name, "pid", cmd.Process.Pid, "process_output_file", logFilePath)

	go p.waitForExit()

	return nil
}

func (p *Proc) Kill() error {
	snapshot := p.Snapshot()
	if snapshot.PID == 0 {
		slog.Warn("no PID set for process; nothing to kill", "name", p.Name)
		return nil
	}

	if snapshot.Status != StatusRunning {
		slog.Info("process already stopped", "name", p.Name, "pid", snapshot.PID, "status", snapshot.Status)
		return nil
	}

	if err := terminateManagedProcess(snapshot.PID); err != nil {
		slog.Error("failed to kill process", "name", p.Name, "pid", snapshot.PID, "error", err)
		return err
	}

	p.mu.Lock()
	p.Status = StatusKilled
	p.closeLogFileLocked()
	p.mu.Unlock()

	slog.Info("process killed", "name", p.Name, "pid", snapshot.PID)
	return nil
}

func (p *Proc) Refresh() Snapshot {
	p.refreshOutput()
	return p.Snapshot()
}

func (p *Proc) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return Snapshot{
		Name:     p.Name,
		PID:      p.PID,
		Status:   p.Status,
		LastLine: p.LastLine,
		Outfile:  p.Outfile,
		ExitCode: p.ExitCode,
	}
}

func (p *Proc) waitForExit() {
	p.mu.RLock()
	cmd := p.Process
	pid := p.PID
	outfile := p.Outfile
	p.mu.RUnlock()

	if cmd == nil {
		return
	}

	err := cmd.Wait()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	p.mu.Lock()
	if p.Status == StatusKilled {
		if exitCode != 0 {
			p.ExitCode = exitCode
		}
		p.closeLogFileLocked()
		p.mu.Unlock()
		slog.Info("process exit observed after kill", "name", p.Name, "pid", pid, "exit_code", exitCode, "process_output_file", outfile)
		return
	}

	p.Status = statusForWait(err)
	p.ExitCode = exitCode
	p.closeLogFileLocked()
	status := p.Status
	p.mu.Unlock()

	if err != nil {
		slog.Warn("process exited with error", "name", p.Name, "pid", pid, "status", status, "exit_code", exitCode, "error", err, "process_output_file", outfile)
		return
	}

	slog.Info("process exited", "name", p.Name, "pid", pid, "status", status, "exit_code", exitCode, "process_output_file", outfile)
}

func statusForWait(err error) string {
	if err == nil {
		return StatusExited
	}
	return StatusFailed
}

func (p *Proc) closeLogFileLocked() {
	if p.LogFile != nil {
		p.LogFile.Close()
		p.LogFile = nil
	}
}

func (p *Proc) refreshOutput() string {
	snapshot := p.Snapshot()
	if snapshot.Outfile == "" {
		return ""
	}

	f, err := os.Open(snapshot.Outfile)
	if err != nil {
		return ""
	}
	defer f.Close()

	var lastLine string
	stat, err := f.Stat()
	if err != nil || stat.Size() == 0 {
		return ""
	}

	readSize := min(stat.Size(), int64(4096))
	_, _ = f.Seek(-readSize, io.SeekEnd)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if t := scanner.Text(); t != "" {
			lastLine = t
		}
	}

	p.mu.Lock()
	p.LastLine = lastLine
	p.mu.Unlock()

	return lastLine
}

// RefreshOutput reads the last line from the process log file and caches it in LastLine.
func (p *Proc) RefreshOutput() string {
	return p.refreshOutput()
}

type Item struct {
	title       string
	description string
	Process     *Proc
	Deleting    bool
	Frame       string
}

func NewItem(proc *Proc) Item {
	description := ""
	if proc != nil {
		description = proc.Snapshot().Status
	}

	return Item{
		title:       proc.Name,
		description: description,
		Process:     proc,
	}
}

func (i Item) FilterValue() string {
	return i.title
}

func (i Item) Title() string {
	return i.title
}

func (i Item) Description() string {
	return i.description
}

type DelegateItem struct {
	Item
}
