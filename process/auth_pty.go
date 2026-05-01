package process

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
)

type AuthPTYSession struct {
	command string
	cmd     *exec.Cmd
	ptyFile *os.File

	outputCh chan []byte
	exitCh   chan error

	mu     sync.Mutex
	closed bool
}

func StartAuthPTYSession(command string) (*AuthPTYSession, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return nil, fmt.Errorf("auth command is empty")
	}

	cmd := exec.Command(trimmed)
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	session := &AuthPTYSession{
		command:  trimmed,
		cmd:      cmd,
		ptyFile:  f,
		outputCh: make(chan []byte, 128),
		exitCh:   make(chan error, 1),
	}

	go session.readLoop()
	go session.waitLoop()

	return session, nil
}

func (s *AuthPTYSession) Output() <-chan []byte {
	return s.outputCh
}

func (s *AuthPTYSession) Done() <-chan error {
	return s.exitCh
}

func (s *AuthPTYSession) WriteInput(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}

	_, err := s.ptyFile.Write(data)
	return err
}

func (s *AuthPTYSession) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}

	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}

	return pty.Setsize(s.ptyFile, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (s *AuthPTYSession) Interrupt() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	return s.cmd.Process.Signal(os.Interrupt)
}

func (s *AuthPTYSession) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	ptyFile := s.ptyFile
	s.ptyFile = nil
	s.mu.Unlock()

	if ptyFile != nil {
		return ptyFile.Close()
	}

	return nil
}

func (s *AuthPTYSession) readLoop() {
	f := s.ptyFile
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			s.outputCh <- chunk
		}
		if err != nil {
			close(s.outputCh)
			return
		}
	}
}

func (s *AuthPTYSession) waitLoop() {
	err := s.cmd.Wait()
	s.exitCh <- err
	close(s.exitCh)
	_ = s.Close()
}
