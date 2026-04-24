package tui

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Services   []Service `yaml:"services"`
	SourceFile string
}

type Service struct {
	Name    string `yaml:"name"`
	Target  string `yaml:"target"`
	Ports   Ports  `yaml:"ports"`
	Profile string `yaml:"profile"`
	Region  string `yaml:"region"`
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
		fmt.Printf("Error finding config file %s: %s", absPath, err)
		return nil, err
	}

	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("Error reading config file %s: %s", file, err)
		return nil, err
	}

	return data, nil
}
