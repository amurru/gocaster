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
	if err != nil {
		return err
	}

	// Create downloads table
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS downloads (
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
        )
    `)
	if err != nil {
		return err
	}

	// Create indexes for downloads table
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_downloads_updated_at ON downloads(updated_at)`)
	return err
}
