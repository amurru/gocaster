package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/amurru/gocaster/internal/application"
	"github.com/amurru/gocaster/internal/domain"
	"github.com/amurru/gocaster/internal/infrastructure/config"
	"github.com/amurru/gocaster/internal/infrastructure/logging"
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

	// Build the structured logger (issue #14). It writes to stderr — never
	// stdout, which the TUI owns — so diagnostics can never corrupt the
	// rendered interface. DEBUG=true enables debug-level records, preserving
	// the previous opt-in behavior.
	level := slog.LevelInfo
	if len(os.Getenv("DEBUG")) > 0 {
		level = slog.LevelDebug
	}
	logger := logging.NewLogger(level)

	cfg, err := config.LoadOrCreate(logger)
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

	// Root context: cancelled on SIGINT (Ctrl-C) or SIGTERM (e.g. systemd
	// stop), and again explicitly at the end of run(). Every service call and
	// every in-flight download goroutine derives from it, so cancellation
	// propagates into the DB (ExecContext), the RSS fetch, the HTTP download
	// body, and the player (issue #11). It is also passed to tea.NewProgram so
	// the TUI's event loop observes cancellation and exits (CodeRabbit review
	// of PR #41). defer cancelRoot is the catch-all that runs on every return
	// path, including the p.Run() error path.
	ctx, cancelRoot := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelRoot()

	fetcher := rss.NewFeedFetcher()
	// Services accept only the focused repository ports they use (issue #17);
	// repo (SQLiteRepo) satisfies the union PodcastRepository, hence each port.
	podcastSvc := application.NewPodcastService(repo, repo, repo, fetcher, logger)

	// Handle CLI subcommands before starting the TUI.
	args := os.Args
	if len(args) > 1 {
		switch args[1] {
		case "export-opml":
			if len(args) != 3 {
				return fmt.Errorf("usage: gocaster export-opml <file>")
			}
			return podcastSvc.ExportSubscriptions(ctx, args[2])

		case "import-opml":
			if len(args) != 3 {
				return fmt.Errorf("usage: gocaster import-opml <file>")
			}
			result, err := podcastSvc.ImportSubscriptions(ctx, args[2])
			if err != nil {
				return err
			}
			fmt.Printf("Imported %d subscriptions (skipped %d, failed %d)\n", result.Added, result.Skipped, result.Failed)
			for _, f := range result.Failures {
				fmt.Fprintf(os.Stderr, "  failed: %s — %v\n", f.FeedURL, f.Err)
			}
			return nil
		}
	}

	downloadSvc := application.NewDownloadService(repo, repo, repo, repo, cfg.DownloadPath, logger)
	// Parent every per-job download context from the root so cancelling the
	// root (shutdown / Ctrl-C) cancels every in-flight download (issue #11).
	downloadSvc.SetRootContext(ctx)

	// Setup player and broadcaster
	mpvPlayer := player.NewMPVPlayer(
		player.WithAudioOutput(cfg.AudioOutput),
		player.WithLogger(logger),
	)
	mprisBroadcaster, err := system.NewMPRISBroadcaster()
	if err != nil {
		logger.Warn("failed to create MPRIS broadcaster", "err", err)
	}
	broadcasters := []domain.PlaybackBroadcaster{mprisBroadcaster}
	if cfg.DiscordPresence {
		discordBroadcaster, discordErr := system.NewDiscordBroadcaster(cfg.DiscordClientID)
		if discordErr != nil {
			logger.Warn("failed to create Discord broadcaster", "err", discordErr)
		} else {
			broadcasters = append(broadcasters, discordBroadcaster)
		}
	}
	broadcaster := system.NewCompositeBroadcaster(broadcasters...)
	queueSvc := application.NewQueueService(repo, repo, repo, mpvPlayer, broadcaster, logger)
	playerSvc := application.NewPlayerService(repo, repo, mpvPlayer, broadcaster, logger, queueSvc)

	// Get custom themes directory
	customThemesDir, err := config.GetCustomThemesDir()
	if err != nil {
		logger.Warn("failed to determine custom themes directory", "err", err)
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
	model := tui.NewModel(ctx, podcastSvc, downloadSvc, playerSvc, queueSvc, settings, saveSettings, customThemesDir)

	// tea.WithContext makes the program's event loop observe ctx: when the root
	// context is cancelled (SIGINT/SIGTERM), the loop exits and p.Run()
	// returns, instead of relying solely on the terminal's Ctrl-C handling
	// (CodeRabbit review of PR #41).
	p := tea.NewProgram(model, tea.WithContext(ctx))
	// The previously bare tea.Quit() call after Run() was a no-op: tea.Quit is
	// only meaningful as a command returned from Update, and Run() has already
	// returned. Cleanup happens via the deferred calls below and the root
	// context cancellation above.
	_, runErr := p.Run()

	// --- Shutdown sequence (issue #11) -------------------------------------
	// 1. Cancel the root context. This cancels every in-flight download
	//    goroutine's per-job context (derived from the root) and any service
	//    call still holding ctx.
	cancelRoot()
	// 2. Cancel any in-flight downloads directly (defence in depth: covers
	//    goroutines whose ctx was a background child, e.g. tests). The
	//    goroutines observe the cancellation on their next read and fail the
	//    job with partial bytes preserved (issue #1).
	_ = downloadSvc.Shutdown(ctx)
	// 3. Close the player service (which closes the broadcaster and the
	//    player) before repo.Close(), since shutdown may persist final playback
	//    state. Runs after the root is cancelled so broadcaster/player calls
	//    inside Close observe cancellation (issue #9 ordering).
	_ = playerSvc.Close(ctx)

	return runErr
}
