// internal/infrastructure/persistence/migrations.go
package persistence

import (
	"database/sql"
	"fmt"
)

// migration represents a single versioned schema migration.
type migration struct {
	version int
	name    string
	up      []string
}

// migrations is the ordered list of all schema migrations.
// Add new migrations at the end - never modify existing entries.
var migrations = []migration{
	{
		version: 1,
		name:    "initial_schema",
		up: []string{
			`CREATE TABLE IF NOT EXISTS podcasts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				title TEXT NOT NULL,
				feed_url TEXT NOT NULL UNIQUE,
				description TEXT,
				image_url TEXT,
				last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS episodes (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				podcast_id INTEGER NOT NULL,
				title TEXT NOT NULL,
				description TEXT,
				audio_url TEXT NOT NULL,
				published_at DATETIME,
				playback_duration INTEGER DEFAULT 0,
				is_played BOOLEAN DEFAULT 0,
				is_downloaded BOOLEAN DEFAULT 0,
				local_path TEXT,
				FOREIGN KEY (podcast_id) REFERENCES podcasts(id) ON DELETE CASCADE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_episodes_podcast_id ON episodes(podcast_id)`,
			`CREATE INDEX IF NOT EXISTS idx_episodes_published_at ON episodes(published_at DESC)`,
			`CREATE TABLE IF NOT EXISTS downloads (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				episode_id INTEGER NOT NULL UNIQUE,
				status TEXT NOT NULL DEFAULT 'queued',
				bytes_downloaded INTEGER DEFAULT 0,
				bytes_total INTEGER DEFAULT 0,
				temp_path TEXT,
				final_path TEXT,
				etag TEXT,
				last_modified TEXT,
				supports_resume BOOLEAN DEFAULT 0,
				error_message TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (episode_id) REFERENCES episodes(id) ON DELETE CASCADE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status)`,
			`CREATE INDEX IF NOT EXISTS idx_downloads_updated_at ON downloads(updated_at)`,
		},
	},
	// Future migrations go here, e.g.:
	// {
	// 	version: 2,
	// 	name:    "add_episode_guid",
	// 	up: []string{
	// 		`ALTER TABLE episodes ADD COLUMN guid TEXT`,
	// 		`CREATE UNIQUE INDEX IF NOT EXISTS idx_episodes_guid ON episodes(guid) WHERE guid IS NOT NULL`,
	// 	},
	// },
	{
		version: 2,
		name:    "add_feed_conditional_headers",
		up: []string{
			`ALTER TABLE podcasts ADD COLUMN etag TEXT`,
			`ALTER TABLE podcasts ADD COLUMN last_modified TEXT`,
		},
	},
	{
		version: 3,
		name:    "add_download_sha256",
		up: []string{
			`ALTER TABLE downloads ADD COLUMN sha256 TEXT DEFAULT ''`,
		},
	},
}

// RunMigrations applies all pending schema migrations to the database.
// It creates the schema_migrations table if it doesn't exist, then
// applies each pending migration in a transaction. Existing databases
// from before this migration system are handled by detecting the absence
// of schema_migrations and recording version 1 as already applied.
func RunMigrations(db *sql.DB) error {
	// Create schema_migrations table
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	// Check if this is an existing database (has tables but no schema_migrations)
	if existing, err := hasExistingDB(db); err != nil {
		return fmt.Errorf("check existing schema: %w", err)
	} else if existing {
		// Existing DB from before migration versioning - mark version 1 as applied
		if _, err := db.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (1)`); err != nil {
			return fmt.Errorf("mark existing schema as version 1: %w", err)
		}
	}

	// Get applied versions
	applied, err := getAppliedVersions(db)
	if err != nil {
		return fmt.Errorf("get applied versions: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", m.version, m.name, err)
		}
	}

	return nil
}

// hasExistingSchema checks if this is an existing database that was created
// before the migration versioning system was introduced. It returns true if
// the podcasts table exists but schema_migrations does not.
func hasExistingDB(db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='podcasts'`).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// getAppliedVersions returns a set of all migration versions already applied.
func getAppliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// applyMigration runs all statements for a migration in a single transaction.
// If any statement fails, the entire migration is rolled back.
func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range m.up {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}

	// Record the migration version
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
		return fmt.Errorf("record version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
