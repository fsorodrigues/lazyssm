package process

import (
	"log"
	"os"
	"os/exec"
)

type Proc struct {
	Name    string
	Cmd     string
	PID     int
	Outfile string
	Status  string
	Process *exec.Cmd
	LogFile *os.File
}

func (p *Proc) Run() {
	// 1. Create or open the log file
	logFile, err := os.OpenFile("process.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
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
	log.Printf("Background process started with PID: %d. Logging to process.log\n", p.PID)
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
