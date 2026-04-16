# Gocaster

Gocaster is a lightweight, terminal-based podcast client written in Go using the Bubble Tea TUI framework.

This repository contains the application, adapters for persistence and playback, and a Bubble Tea-based TUI so you can browse, subscribe, and play podcast episodes from your terminal.

## Key highlights

- Terminal UI (TUI) built with charm.land/bubbletea
- RSS/Atom feed parsing via github.com/mmcdole/gofeed
- SQLite persistence (github.com/mattn/go-sqlite3)
- MPV adapter for audio playback (using go-mpv bindings; mpv must be installed separately)
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

### Database

The app uses an SQLite database (gocaster.db) created on first run. Schema is managed by the SQLite repository implementation under internal/infrastructure. If you need to reset the database, remove gocaster.db from the working directory and restart the app.

## Usage

- Start the app and use the interactive TUI to add podcast feeds, browse episodes, and control playback.
- Inline filtering: press `/` to start filtering lists (library and episode lists support filtering by title).

## Configuration

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
