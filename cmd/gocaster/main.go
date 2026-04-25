package main

import (
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/domain"
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
	mprisBroadcaster, err := system.NewMPRISBroadcaster()
	if err != nil {
		log.Printf("Warning: failed to create MPRIS broadcaster: %v", err)
	}
	broadcasters := []domain.PlaybackBroadcaster{mprisBroadcaster}
	if cfg.DiscordPresence {
		discordBroadcaster, discordErr := system.NewDiscordBroadcaster(cfg.DiscordClientID)
		if discordErr != nil {
			log.Printf("Warning: failed to create Discord broadcaster: %v", discordErr)
		} else {
			broadcasters = append(broadcasters, discordBroadcaster)
		}
	}
	broadcaster := system.NewCompositeBroadcaster(broadcasters...)
	playerSvc := application.NewPlayerService(repo, mpvPlayer, broadcaster)

	// Get custom themes directory
	customThemesDir, err := config.GetCustomThemesDir()
	if err != nil {
		log.Printf("Warning: failed to determine custom themes directory: %v", err)
		customThemesDir = ""
	}

	// UI model
	settings := tui.Settings{
		AutoSyncOnStartup: cfg.AutoSyncOnStartup,
		PeriodicSync:      cfg.PeriodicSync,
		PeriodicSyncMins:  cfg.PeriodicSyncMins,
		DiscordPresence:   cfg.DiscordPresence,
		DiscordClientID:   cfg.DiscordClientID,
		ThemeName:         cfg.ThemeName,
	}
	saveSettings := func(next tui.Settings) error {
		cfg.AutoSyncOnStartup = next.AutoSyncOnStartup
		cfg.PeriodicSync = next.PeriodicSync
		cfg.PeriodicSyncMins = next.PeriodicSyncMins
		cfg.DiscordPresence = next.DiscordPresence
		cfg.DiscordClientID = next.DiscordClientID
		cfg.ThemeName = next.ThemeName
		return config.Save(cfg)
	}
	model := tui.NewModel(podcastSvc, downloadSvc, playerSvc, settings, saveSettings, customThemesDir)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("[☠️] there's been an error: %v", err)
		os.Exit(1)
	}
	_ = playerSvc.Close()
	tea.Quit()
}
