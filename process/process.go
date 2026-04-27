package process

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

type Proc struct {
	Name     string
	Cmd      string
	PID      int
	Outfile  string
	Status   string
	Process  *exec.Cmd
	LogFile  *os.File
	LastLine string
}

func (p *Proc) Run() {
	// 1. Create or open the log file
	logFileName := fmt.Sprintf("process-%s.log", p.Name)
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if err != nil {
		panic(err)
	}

	// 2. Define command
	cmd := exec.Command("./simulate_service.sh", p.Name, "5")

	// 3. Set output to the log file
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// 4. Start the command (does not wait for completion)
	if err := cmd.Start(); err != nil {
		logFile.Close()
		panic(err)
	}

	// 5. Store references for later cleanup
	p.Process = cmd
	p.PID = cmd.Process.Pid
	p.Status = "running"
	p.LogFile = logFile
	p.Outfile = logFileName
	log.Printf("Background process started with PID: %d. Logging to %s\n", p.PID, logFileName)
}

func (p Proc) Kill() {
	if p.PID == 0 {
		log.Printf("No PID set for process %q; nothing to kill\n", p.Name)
		return
	}

	proc, err := os.FindProcess(p.PID)
	if err != nil {
		log.Printf("Could not find process %d: %v\n", p.PID, err)
		return
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		log.Printf("Failed to send SIGINT to %d, attempting SIGKILL: %v\n", p.PID, err)
		if err := proc.Kill(); err != nil {
			log.Printf("Failed to kill process %d: %v\n", p.PID, err)
		}
	}

	log.Printf("Process %d (%s) killed\n", p.PID, p.Name)

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

	// Seek to the end and scan backwards for the last non-empty line.
	var lastLine string
	stat, err := f.Stat()
	if err != nil || stat.Size() == 0 {
		return ""
	}

	// Read from end, up to 4KB should be enough for last line
	readSize := int64(4096)
	if stat.Size() < readSize {
		readSize = stat.Size()
	}
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
