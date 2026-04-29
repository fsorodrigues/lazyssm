package tui

import (
	"log/slog"
	"os"
	"path/filepath"
)

type Config struct {
	Services      []Service `yaml:"services"`
	SourceFile    string
	ProcessLogDir string
}

type Service struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Target      string `yaml:"target"`
	Ports       Ports  `yaml:"ports"`
	Profile     string `yaml:"profile"`
	Region      string `yaml:"region"`
}

type Ports struct {
	Port      int `yaml:"port"`
	LocalPort int `yaml:"localPort"`
}

func NewConfig() *Config {
	return &Config{}
}

func ReadConfigFromFile(file string) ([]byte, error) {
	absPath, err := filepath.Abs(file)
	if err != nil {
		slog.Error("resolve config file path", "path", file, "resolved_path", absPath, "error", err)
		return nil, err
	}

	slog.Debug("reading config file", "path", file, "resolved_path", absPath)
	data, err := os.ReadFile(file)
	if err != nil {
		slog.Error("read config file", "path", file, "error", err)
		return nil, err
	}

	return data, nil
}
