package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestUserStateDir(t *testing.T) {
	dir, err := userStateDir()
	if err != nil {
		t.Fatalf("userStateDir failed: %v", err)
	}

	if dir == "" {
		t.Fatal("userStateDir returned empty string")
	}

	expectedDefault := filepath.Join(os.Getenv("HOME"), ".local", "state")
	if dir != expectedDefault && os.Getenv("XDG_STATE_HOME") == "" {
		t.Logf("expected default %s, got %s", expectedDefault, dir)
	}
}

func TestResolvePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		home     string
	}{
		{"~/Downloads", "", "expected resolved path"},
		{"~/Documents", "", "expected resolved path"},
		{"/absolute/path", "/absolute/path", "expected unchanged"},
		{"relative", "relative", "expected unchanged"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := resolvePath(tt.input)
			if tt.input == "~/Downloads" || tt.input == "~/Documents" {
				home, _ := os.UserHomeDir()
				expected := filepath.Join(home, tt.input[2:])
				if result != expected {
					t.Errorf("resolvePath(%q) = %q, want %q", tt.input, result, expected)
				}
			} else if result != tt.expected {
				t.Errorf("resolvePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsAbsolute(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/home/user/file", true},
		{"/absolute/path", true},
		{"relative/path", false},
		{"./relative", false},
		{"~/Downloads", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isAbsolute(tt.path)
			if result != tt.expected {
				t.Errorf("isAbsolute(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestConfigFields(t *testing.T) {
	cfg := Config{
		DatabasePath: "/custom/db/path.db",
		DownloadPath: "/custom/downloads",
	}

	meta, err := toml.Decode(`
database_path = "/custom/db/path.db"
download_path = "/custom/downloads"
`, &cfg)
	if err != nil {
		t.Fatalf("toml decode failed: %v", err)
	}

	if cfg.DatabasePath != "/custom/db/path.db" {
		t.Errorf("database_path = %q, want %q", cfg.DatabasePath, "/custom/db/path.db")
	}
	if cfg.DownloadPath != "/custom/downloads" {
		t.Errorf("download_path = %q, want %q", cfg.DownloadPath, "/custom/downloads")
	}

	if len(meta.Undecoded()) > 0 {
		t.Errorf("unexpected undecoded fields: %v", meta.Undecoded())
	}
}

func TestConfigPartialFields(t *testing.T) {
	cfg := Config{}

	_, err := toml.Decode(`
database_path = "/only/db"
`, &cfg)
	if err != nil {
		t.Fatalf("toml decode failed: %v", err)
	}

	if cfg.DatabasePath != "/only/db" {
		t.Errorf("database_path = %q, want %q", cfg.DatabasePath, "/only/db")
	}
	if cfg.DownloadPath != "" {
		t.Errorf("download_path = %q, want empty (default)", cfg.DownloadPath)
	}
}

func TestWriteDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gocaster.toml")

	cfg := Config{
		DatabasePath: "/default/db.db",
		DownloadPath: "/default/downloads",
	}

	err := writeDefaultConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("writeDefaultConfig failed: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file failed: %v", err)
	}

	if string(content) == "" {
		t.Error("config file is empty")
	}

	var loaded Config
	_, err = toml.Decode(string(content), &loaded)
	if err != nil {
		t.Fatalf("config file is not valid TOML: %v", err)
	}

	if loaded.DatabasePath != cfg.DatabasePath {
		t.Errorf("database_path in file = %q, want %q", loaded.DatabasePath, cfg.DatabasePath)
	}
	if loaded.DownloadPath != cfg.DownloadPath {
		t.Errorf("download_path in file = %q, want %q", loaded.DownloadPath, cfg.DownloadPath)
	}
}
