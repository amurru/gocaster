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
	// run() holds all setup and the program loop so that deferred cleanup
	// (repo.Close, playerSvc.Close) actually unwinds before os.Exit. A bare
	// os.Exit(1) inside the body would skip the defers, defeating the
	// resource-cleanup fix (issue #9); returning an error lets main exit
	// non-zero only after defers have run.
	if err := run(); err != nil {
		fmt.Printf("[☠️] there's been an error: %v", err)
		os.Exit(1)
	}
}

func run() error {
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
	// Ensure the SQLite handle is released on every exit path (issue #9). It was
	// previously never closed, leaking the DB handle.
	defer repo.Close()

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
	// Close the player service (which closes the broadcaster and the player) on
	// every exit path, before repo.Close() runs via its deferred call (issue #9).
	// playerSvc.Close must run before the DB is closed since shutdown may persist
	// final playback state, so defer it after the repo defer (defers are LIFO).
	defer playerSvc.Close()

	p := tea.NewProgram(model)
	// The previously bare tea.Quit() call after Run() was a no-op: tea.Quit is
	// only meaningful as a command returned from Update, and Run() has already
	// returned. Cleanup happens via the deferred Close calls above.
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
