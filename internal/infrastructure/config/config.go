package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DatabasePath      string `toml:"database_path"`
	DownloadPath      string `toml:"download_path"`
	AutoSyncOnStartup bool   `toml:"auto_sync_on_startup"`
	PeriodicSync      bool   `toml:"periodic_sync_enabled"`
	PeriodicSyncMins  int    `toml:"periodic_sync_minutes"`
	DiscordPresence   bool   `toml:"discord_presence_enabled"`
	DiscordClientID   string `toml:"discord_client_id"`
	ThemeName         string `toml:"theme_name"`
}

const defaultPeriodicSyncMins = 60
const defaultThemeName = "dark-red"

const (
	// DefaultDiscordClientID is the official Gocaster Discord application ID.
	// Users can override it from config/TUI settings.
	DefaultDiscordClientID = "1496999428605612203"
	// DefaultDiscordPublicKey is the official Gocaster Discord application public key.
	// Kept for reference; Discord Rich Presence runtime only requires client ID.
	DefaultDiscordPublicKey = "b6910d4eead9b118c44fad8079475c5f51aefc362100fdd62b9c14e30f6893fb"
)

func LoadOrCreate() (Config, error) {
	dirs, err := getDirs()
	if err != nil {
		return Config{}, fmt.Errorf("failed to determine config dirs: %w", err)
	}

	configPath := filepath.Join(dirs.configDir, "gocaster.toml")

	cfg := Config{
		DatabasePath:      dirs.defaultDB,
		DownloadPath:      dirs.defaultDownloads,
		AutoSyncOnStartup: false,
		PeriodicSync:      false,
		PeriodicSyncMins:  defaultPeriodicSyncMins,
		DiscordPresence:   false,
		DiscordClientID:   DefaultDiscordClientID,
		ThemeName:         defaultThemeName,
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := writeDefaultConfig(configPath, cfg); err != nil {
			fmt.Printf("Warning: could not create default config: %v\n", err)
		}
		return cfg, nil
	}

	meta, err := toml.DecodeFile(configPath, &cfg)
	if err != nil {
		fmt.Printf("Warning: config file malformed, using defaults: %v\n", err)
		return cfg, nil
	}

	if cfg.DatabasePath == "" {
		cfg.DatabasePath = dirs.defaultDB
	} else {
		cfg.DatabasePath = resolvePath(cfg.DatabasePath)
		if !isAbsolute(cfg.DatabasePath) {
			fmt.Printf("Warning: database_path is not absolute, using default\n")
			cfg.DatabasePath = dirs.defaultDB
		}
	}

	if cfg.DownloadPath == "" {
		cfg.DownloadPath = dirs.defaultDownloads
	} else {
		cfg.DownloadPath = resolvePath(cfg.DownloadPath)
		if !isAbsolute(cfg.DownloadPath) {
			fmt.Printf("Warning: download_path is not absolute, using default\n")
			cfg.DownloadPath = dirs.defaultDownloads
		}
	}

	if len(meta.Undecoded()) > 0 {
		fmt.Printf("Warning: config has unknown fields, ignoring them\n")
	}

	if cfg.PeriodicSyncMins <= 0 {
		fmt.Printf("Warning: periodic_sync_minutes must be greater than zero, using default\n")
		cfg.PeriodicSyncMins = defaultPeriodicSyncMins
	}
	cfg.DiscordClientID = strings.TrimSpace(cfg.DiscordClientID)
	if cfg.DiscordPresence && cfg.DiscordClientID == "" {
		fmt.Printf("Warning: discord_presence_enabled requires discord_client_id, disabling Discord presence\n")
		cfg.DiscordPresence = false
	}

	if cfg.ThemeName == "" {
		cfg.ThemeName = defaultThemeName
	}

	return cfg, nil
}

type dirs struct {
	configDir        string
	stateDir         string
	defaultDB        string
	defaultDownloads string
}

func getDirs() (dirs, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return dirs{}, fmt.Errorf("could not determine user config dir: %w", err)
	}

	stateDir, err := userStateDir()
	if err != nil {
		return dirs{}, fmt.Errorf("could not determine user state dir: %w", err)
	}

	return dirs{
		configDir:        filepath.Join(configDir, "gocaster"),
		stateDir:         filepath.Join(stateDir, "gocaster"),
		defaultDB:        filepath.Join(stateDir, "gocaster", "gocaster.db"),
		defaultDownloads: filepath.Join(stateDir, "gocaster", "downloads"),
	}, nil
}

func userStateDir() (string, error) {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		return stateHome, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state"), nil
}

func resolvePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	return path
}

func isAbsolute(path string) bool {
	return filepath.IsAbs(path)
}

func writeDefaultConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not create config file: %w", err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("could not encode default config: %w", err)
	}

	return nil
}

func Save(cfg Config) error {
	if cfg.PeriodicSyncMins <= 0 {
		return fmt.Errorf("periodic_sync_minutes must be greater than zero")
	}
	cfg.DiscordClientID = strings.TrimSpace(cfg.DiscordClientID)
	if cfg.DiscordPresence && cfg.DiscordClientID == "" {
		return fmt.Errorf("discord_client_id is required when discord_presence_enabled is true")
	}
	dirs, err := getDirs()
	if err != nil {
		return fmt.Errorf("failed to determine config dirs: %w", err)
	}
	configPath := filepath.Join(dirs.configDir, "gocaster.toml")
	return writeDefaultConfig(configPath, cfg)
}

func EnsureDirs(cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0755); err != nil {
		return fmt.Errorf("could not create database directory: %w", err)
	}

	if err := os.MkdirAll(cfg.DownloadPath, 0755); err != nil {
		return fmt.Errorf("could not create download directory: %w", err)
	}

	return nil
}

// GetCustomThemesDir returns the directory where custom themes are stored
func GetCustomThemesDir() (string, error) {
	dirs, err := getDirs()
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs.configDir, "themes"), nil
}
