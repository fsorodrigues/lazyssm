package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultLogFile       = "~/.config/lazyssm/log/lazyssm.log"
	DefaultProcessLogDir = "~/.local/state/lazyssm/process"
)

func Setup(logPath string, debug bool) (*os.File, string, error) {
	resolvedPath, err := ExpandPath(logPath)
	if err != nil {
		return nil, "", fmt.Errorf("expand log path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return nil, "", fmt.Errorf("create log directory: %w", err)
	}

	logFile, err := os.OpenFile(resolvedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("open log file: %w", err)
	}

	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))

	return logFile, resolvedPath, nil
}

func EnsureDir(path string) (string, error) {
	resolvedPath, err := ExpandPath(path)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(resolvedPath, 0o755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	return resolvedPath, nil
}

func ConfigureTeaDebugEnv(debug bool, logPath string) func() {
	originalTrace, hadTrace := os.LookupEnv("TEA_TRACE")
	originalDebug, hadDebug := os.LookupEnv("TEA_DEBUG")

	if debug {
		_ = os.Setenv("TEA_TRACE", logPath)
		_ = os.Setenv("TEA_DEBUG", "true")
	} else {
		_ = os.Unsetenv("TEA_TRACE")
		_ = os.Unsetenv("TEA_DEBUG")
	}

	return func() {
		if hadTrace {
			_ = os.Setenv("TEA_TRACE", originalTrace)
		} else {
			_ = os.Unsetenv("TEA_TRACE")
		}

		if hadDebug {
			_ = os.Setenv("TEA_DEBUG", originalDebug)
		} else {
			_ = os.Unsetenv("TEA_DEBUG")
		}
	}
}

func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("log path is empty")
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if path == "~" {
			return homeDir, nil
		}
		path = filepath.Join(homeDir, path[2:])
	}

	return filepath.Abs(path)
}
