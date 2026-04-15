package main

import (
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/infrastructure/persistence"
	"github.com/amurru/gocaster/internal/infrastructure/rss"
	"github.com/amurru/gocaster/internal/interface/tui"
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
	dbPath := "gocaster.db" // TODO: make configurable
	repo, err := persistence.NewSQLiteRepo(dbPath)
	if err != nil {
		log.Fatal("fatal: ", err)
	}
	fetcher := rss.NewFeedFetcher()
	podcastSvc := application.NewPodcastService(repo, fetcher)

	// UI model
	model := tui.NewModel(podcastSvc)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("[☠️] there's been an error: %v", err)
		os.Exit(1)
	}
	tea.Quit()
}
