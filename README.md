# Gocaster

![Gocaster](assets/gocaster.png)

Gocaster is a lightweight, terminal-based podcast client written in Go using the Bubble Tea TUI framework.

This repository contains the application, adapters for persistence and playback, and a Bubble Tea-based TUI so you can browse, subscribe, and play podcast episodes from your terminal.

## Screenshot

![Gocaster Screenshot](assets/screenshot.png)

## Key highlights

- Terminal UI (TUI) built with charm.land/bubbletea
- RSS/Atom feed parsing via github.com/mmcdole/gofeed
- SQLite persistence (github.com/mattn/go-sqlite3)
- MPV adapter for audio playback (using go-mpv bindings; mpv must be installed separately)
- MPRIS support for desktop media controls (Linux)
- Download queue with episode downloading
- Clean architecture: domain, application, infrastructure, interface layers

## Contents

- cmd/gocaster/ - application entrypoint and dependency wiring
- internal/domain/ - core entities and port interfaces (Podcast, Episode, Repository, Player)
- internal/application/ - services / use-cases (PodcastService, PlayerService)
- internal/infrastructure/ - adapters: sqlite persistence, mpv player, RSS fetcher
- internal/interface/tui/ - Bubble Tea UI implementation and components

## Quick start

### Prerequisites

- Go 1.18+
- mpv (optional for audio playback)

### Build and run

- Build the binary: make build (writes to bin/gocaster)
- Run the app: make run
- Run with debug logging: make debug-run (writes debug.log)

### Testing and quality

- Run tests: make test
- Run tests with coverage: make test-coverage (produces coverage.html)
- Lint: make lint
- Format: make format
- Vet: make vet
- Full quality check: make check

### Configuration

Gocaster reads configuration from a TOML file. The default config file location is:

- Linux: `~/.config/gocaster/gocaster.toml` (or `$XDG_CONFIG_HOME/gocaster/gocaster.toml` if set)

If the config file doesn't exist, it will be created automatically with default values.

**Config options:**

| Option                     | Default                               | Description                                                              |
| -------------------------- | ------------------------------------- | ------------------------------------------------------------------------ |
| `database_path`            | `~/.local/state/gocaster/gocaster.db` | Full path to SQLite database                                             |
| `download_path`            | `~/.local/state/gocaster/downloads`   | Full path to download directory                                          |
| `auto_sync_on_startup`     | `false`                               | Refresh all subscribed podcasts when app starts                          |
| `periodic_sync_enabled`    | `false`                               | Enable automatic periodic refresh while app is open                      |
| `periodic_sync_minutes`    | `60`                                  | Interval in minutes for periodic refresh (must be > 0)                   |
| `discord_presence_enabled` | `false`                               | Publish current playback as Discord Rich Presence                        |
| `discord_client_id`        | `1496999428605612203`                 | Discord Application Client ID (override if you want to use your own app) |

**Example config:**

```toml
database_path = "/home/user/my-podcasts.db"
download_path = "/home/user/Downloads/Podcasts"
auto_sync_on_startup = true
periodic_sync_enabled = true
periodic_sync_minutes = 60
discord_presence_enabled = true
discord_client_id = "1496999428605612203"
```

You can use `~` in paths (e.g., `~/Downloads`), which will be expanded to your home directory.

If a path is not absolute (after resolving `~`), the default will be used and a warning will be logged.

### Discord Rich Presence setup

To show now-playing activity in Discord:

1. Create a Discord application in the Discord Developer Portal.
2. Copy its **Client ID** into `discord_client_id` (optional if using Gocaster's default app ID).
3. Set `discord_presence_enabled = true`.

If Discord is not running or IPC is unavailable, playback continues normally and presence updates are skipped.

Gocaster ships with a baked-in default Discord application:

- Client ID: `1496999428605612203`
- Public key: `b6910d4eead9b118c44fad8079475c5f51aefc362100fdd62b9c14e30f6893fb`

### Themes

Gocaster includes 14 beautiful predefined themes and supports custom themes. All themes can be selected from Settings (press `S` in the app).

**Predefined Themes:**

- Dark variants: dark-red, dark-orange, dark-yellow, dark-green, dark-blue, dark-indigo, dark-violet
- Light variants: light-red, light-orange, light-yellow, light-green, light-blue, light-indigo, light-violet

**Creating Custom Themes:**

Custom themes can be created as TOML files in `~/.config/gocaster/themes/` and will automatically appear in the theme selector.

**Custom Theme Fields:**

All colors should be specified as 6-digit hex codes. The following fields are available:

- `name` - Display name for the theme (required)
- `background` - Main background color (required)
- `text` - Primary text color (required)
- `surface` - Secondary surface color
- `surface_alt` - Alternative surface color
- `border` - Border color
- `accent` - Primary accent color
- `accent_soft` - Softer accent color for secondary highlights
- `success` - Color for success messages
- `danger` - Color for error messages
- `warning` - Color for warning messages

**Example Custom Themes:**

Minimalist Dark Theme (`~/.config/gocaster/themes/minimalist.toml`):

```toml
[theme]
name = "minimalist"
background = "#0a0a0a"
surface = "#1a1a1a"
surface_alt = "#242424"
border = "#404040"
accent = "#00d4ff"
accent_soft = "#33e5ff"
text = "#e0e0e0"
muted = "#808080"
success = "#00d946"
danger = "#ff4444"
warning = "#ffaa00"
```

High Contrast Light Theme (`~/.config/gocaster/themes/high-contrast.toml`):

```toml
[theme]
name = "high-contrast"
background = "#ffffff"
surface = "#f5f5f5"
surface_alt = "#ebebeb"
border = "#cccccc"
accent = "#0066cc"
accent_soft = "#3399ff"
text = "#000000"
muted = "#666666"
success = "#009900"
danger = "#cc0000"
warning = "#cc6600"
```

### Media Player

- MPV: ensure mpv is installed and available in PATH for the MPV player adapter to work. If you prefer another player, implement the Player domain port and wire it in cmd/gocaster.

## Architecture notes

Gocaster follows dependency inversion: domain interfaces define behavior; application services orchestrate use-cases; infrastructure implements ports and the TUI is an adapter. This makes swapping persistence or player adapters straightforward.

## Contributing

- Bug reports and feature requests: please open an issue.
- Pull requests: fork, create a branch, and open a PR with a clear description and tests where appropriate.
- Coding style: follow gofmt and run `make lint` before submitting.

## License

This project is released under the MIT License. See LICENSE for details.

## Acknowledgements

- Bubble Tea, Bubbles and Lipgloss for the TUI stack
- gofeed for RSS/Atom parsing

If you have questions or would like help extending the project (new player backend, alternate persistence, or UI enhancements), open an issue or PR — contributions are welcome!
