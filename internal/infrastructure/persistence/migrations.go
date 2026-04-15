// internal/infrastructure/persistence/migrations.go
package persistence

import (
	"database/sql"
)

// RunMigrations creates the database schema and runs any pending migrations
func RunMigrations(db *sql.DB) error {
	// Create podcasts table
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS podcasts (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT NOT NULL,
            feed_url TEXT NOT NULL UNIQUE,
            description TEXT,
            image_url TEXT,
            last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		return err
	}

	// Create episodes table
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS episodes (
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
        )
    `)
	if err != nil {
		return err
	}

	// Create indexes for common queries
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_episodes_podcast_id ON episodes(podcast_id)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_episodes_published_at ON episodes(published_at DESC)`)
	return err
}
