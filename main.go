package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"lazyssm/internal/logging"
	"lazyssm/model"
	"lazyssm/process"
	"lazyssm/tui"

	tea "charm.land/bubbletea/v2"
	"go.yaml.in/yaml/v4"
)

func main() {
	sourceFile := flag.String("file", "~/.config/lazyssm/config.yaml", "yaml file containing services configuration")
	debug := flag.Bool("debug", false, "enable debug application and Bubble Tea logs")
	logFilePath := flag.String("log-file", logging.DefaultLogFile, "application log file path; does not affect managed process output files")
	processLogDir := flag.String("process-log-dir", logging.DefaultProcessLogDir, "managed process output directory; does not affect the application log file")
	simulate := flag.Bool("simulate", false, "run managed processes using the simulate service script")
	flag.Parse()

	logFile, resolvedLogPath, err := logging.Setup(*logFilePath, *debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up application log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	restoreTeaEnv := logging.ConfigureTeaDebugEnv(*debug, resolvedLogPath)
	defer restoreTeaEnv()

	slog.Info("application logger initialized", "path", resolvedLogPath, "debug", *debug)
	resolvedProcessLogDir, err := logging.EnsureDir(*processLogDir)
	if err != nil {
		slog.Error("set up process log directory", "path", *processLogDir, "error", err)
		fmt.Fprintf(os.Stderr, "Error setting up process log directory: %v\n", err)
		os.Exit(1)
	}

	slog.Debug("parsed CLI flags", "config_file", *sourceFile, "log_file", resolvedLogPath, "process_log_dir", resolvedProcessLogDir)

	configBytes, err := tui.ReadConfigFromFile(*sourceFile)
	if err != nil {
		slog.Error("read config file", "path", *sourceFile, "error", err)
		fmt.Fprintf(os.Stderr, "Error reading config. Existing config file is required: %v\n", err)
		os.Exit(1)
	}

	config := tui.NewConfig()
	err = yaml.Unmarshal(configBytes, config)
	config.SourceFile = *sourceFile
	config.ProcessLogDir = resolvedProcessLogDir
	if err != nil {
		slog.Error("parse config yaml", "path", *sourceFile, "error", err)
		fmt.Fprintf(os.Stderr, "Error parsing config yaml. Valid config yaml is required: %v\n", err)
		os.Exit(1)
	}
	slog.Debug("loaded config", "service_count", len(config.Services), "source_file", config.SourceFile, "process_log_dir", config.ProcessLogDir)

	builder, err := process.NewCommandBuilder(*simulate)
	if err != nil {
		slog.Error("configure command builder", "simulate", *simulate, "error", err)
		fmt.Fprintf(os.Stderr, "Error configuring command builder: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model.InitModel(config, builder))
	if _, err := p.Run(); err != nil {
		slog.Error("run TUI program", "error", err)
		fmt.Fprintf(os.Stderr, "Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}

	slog.Info("application stopped")
}
