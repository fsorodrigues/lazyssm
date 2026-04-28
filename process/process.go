package process

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
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
}

func (p *Proc) Run() {
	logFileName := fmt.Sprintf("process-%s.log", p.Name)
	logFilePath := filepath.Join(p.ProcessLogDir, logFileName)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if err != nil {
		panic(err)
	}

	cmd := exec.Command("./simulate_service.sh", p.Name, "5")
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		panic(err)
	}

	p.Process = cmd
	p.PID = cmd.Process.Pid
	p.Status = "running"
	p.LogFile = logFile
	p.Outfile = logFilePath
	slog.Info("background process started", "name", p.Name, "pid", p.PID, "process_output_file", logFilePath)
}

func (p Proc) Kill() {
	if p.PID == 0 {
		slog.Warn("no PID set for process; nothing to kill", "name", p.Name)
		return
	}

	proc, err := os.FindProcess(p.PID)
	if err != nil {
		slog.Error("could not find process", "name", p.Name, "pid", p.PID, "error", err)
		return
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		slog.Warn("failed to send SIGINT; attempting SIGKILL", "name", p.Name, "pid", p.PID, "error", err)
		if err := proc.Kill(); err != nil {
			slog.Error("failed to kill process", "name", p.Name, "pid", p.PID, "error", err)
		}
	}

	slog.Info("process killed", "name", p.Name, "pid", p.PID)

	if p.LogFile != nil {
		p.LogFile.Close()
		p.LogFile = nil
	}
}

// RefreshOutput reads the last line from the process log file and caches it in LastLine.
func (p *Proc) RefreshOutput() string {
	if p.Outfile == "" {
		return ""
	}

	f, err := os.Open(p.Outfile)
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
	p.LastLine = lastLine
	return lastLine
}

type Item struct {
	title       string
	description string
	Process     *Proc
}

func NewItem(proc *Proc) Item {
	return Item{
		title:       proc.Name,
		description: proc.Status,
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
