package main

import (
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/infrastructure/config"
	"github.com/amurru/gocaster/internal/infrastructure/persistence"
	"github.com/amurru/gocaster/internal/infrastructure/player"
	"github.com/amurru/gocaster/internal/infrastructure/rss"
	"github.com/amurru/gocaster/internal/infrastructure/system"
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

	cfg, err := config.LoadOrCreate()
	if err != nil {
		log.Fatal("fatal: ", err)
	}

	if err := config.EnsureDirs(cfg); err != nil {
		log.Fatal("fatal: ", err)
	}

	repo, err := persistence.NewSQLiteRepo(cfg.DatabasePath)
	if err != nil {
		log.Fatal("fatal: ", err)
	}
	fetcher := rss.NewFeedFetcher()
	podcastSvc := application.NewPodcastService(repo, fetcher)

	downloadSvc := application.NewDownloadService(repo, cfg.DownloadPath)

	// Setup player and broadcaster
	mpvPlayer := player.NewMPVPlayer()
	broadcaster, err := system.NewMPRISBroadcaster()
	if err != nil {
		log.Printf("Warning: failed to create MPRIS broadcaster: %v", err)
	}
	playerSvc := application.NewPlayerService(repo, mpvPlayer, broadcaster)

	// UI model
	model := tui.NewModel(podcastSvc, downloadSvc, playerSvc)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("[☠️] there's been an error: %v", err)
		os.Exit(1)
	}
	_ = playerSvc.Close()
	tea.Quit()
}
