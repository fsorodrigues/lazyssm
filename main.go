package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"lazyssm/model"
	"lazyssm/tui"

	tea "charm.land/bubbletea/v2"
	"go.yaml.in/yaml/v4"
)

func main() {
	sourceFile := flag.String("file", "~/.config/lazyssm/config.yaml", "yaml file containing services configuration")
	flag.Parse()

	configBytes, err := tui.ReadConfigFromFile(*sourceFile)
	if err != nil {
		fmt.Printf("Error reading config. Existing config file is required: %v", err)
		os.Exit(1)
	}

	config := tui.NewConfig()
	err = yaml.Unmarshal(configBytes, config)
	config.SourceFile = *sourceFile
	if err != nil {
		fmt.Printf("Error parsing config yaml. Valid config yaml is required: %v", err)
		os.Exit(1)
	}

	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	}

	p := tea.NewProgram(model.InitModel(config))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	log.Print("bye")
}
