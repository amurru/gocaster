package main

import (
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/amurru/gocaster/internal/app"
)

func main() {
	// Set-up debug logging
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			log.Fatal("fatal: ", err)
		}
		defer f.Close()
	}
	p := tea.NewProgram(app.InitialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
	tea.Quit()
}
