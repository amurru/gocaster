package persistence

import (
	"database/sql"

	"github.com/amurru/gocaster/internal/domain"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteRepo struct {
	db *sql.DB
}

func NewSQLiteRepo(dsn string) (*SQLiteRepo, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	// Run migrations
	if err := RunMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteRepo{db: db}, nil
}

// Close closes the database connection
func (r *SQLiteRepo) Close() error {
	return r.db.Close()
}

func (r *SQLiteRepo) Save(podcast *domain.Podcast) error {
	if podcast.ID == 0 {
		query := `
			INSERT INTO podcasts (title, feed_url, description, image_url, last_updated)
			VALUES (?, ?, ?, ?, ?)
		`
		result, err := r.db.Exec(query, podcast.Title, podcast.FeedURL, podcast.Description, podcast.ImageURL, podcast.LastUpdated)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}

		podcast.ID = id
		return nil
	}

	query := `
		UPDATE podcasts SET title = ?, feed_url = ?, description = ?, image_url = ?, last_updated = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, podcast.Title, podcast.FeedURL, podcast.Description, podcast.ImageURL, podcast.LastUpdated, podcast.ID)
	return err
}

func (r *SQLiteRepo) FindAll() ([]domain.Podcast, error) {
	query := `SELECT id, title, feed_url, description, image_url, last_updated FROM podcasts ORDER BY title`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var podcasts []domain.Podcast
	for rows.Next() {
		var p domain.Podcast
		err := rows.Scan(&p.ID, &p.Title, &p.FeedURL, &p.Description, &p.ImageURL, &p.LastUpdated)
		if err != nil {
			return nil, err
		}
		podcasts = append(podcasts, p)
	}

	return podcasts, rows.Err()
}

func (r *SQLiteRepo) FindByID(id int64) (*domain.Podcast, error) {
	query := `SELECT id, title, feed_url, description, image_url, last_updated FROM podcasts WHERE id = ?`
	var p domain.Podcast
	err := r.db.QueryRow(query, id).Scan(&p.ID, &p.Title, &p.FeedURL, &p.Description, &p.ImageURL, &p.LastUpdated)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *SQLiteRepo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM podcasts WHERE id = ?`, id)
	return err
}

func (r *SQLiteRepo) SaveEpisode(episode *domain.Episode) error {
	query := `
		INSERT INTO episodes (podcast_id, title, description, audio_url, published_at, playback_duration, is_played, is_downloaded, local_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(query, episode.PodcastID, episode.Title, episode.Description, episode.AudioURL, episode.PublishedAt, episode.PlaybackDuration, episode.IsPlayed, episode.IsDownloaded, episode.LocalPath)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	episode.ID = id
	return nil
}

func (r *SQLiteRepo) FindEpisodesByPodcastID(id int64) ([]domain.Episode, error) {
	query := `SELECT id, podcast_id, title, description, audio_url, published_at, playback_duration, is_played, is_downloaded, local_path FROM episodes WHERE podcast_id = ? ORDER BY published_at DESC`
	rows, err := r.db.Query(query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []domain.Episode
	for rows.Next() {
		var e domain.Episode
		err := rows.Scan(&e.ID, &e.PodcastID, &e.Title, &e.Description, &e.AudioURL, &e.PublishedAt, &e.PlaybackDuration, &e.IsPlayed, &e.IsDownloaded, &e.LocalPath)
		if err != nil {
			return nil, err
		}
		episodes = append(episodes, e)
	}

	return episodes, rows.Err()
}

func (r *SQLiteRepo) DeleteEpisode(id int64) error {
	_, err := r.db.Exec(`DELETE FROM episodes WHERE id = ?`, id)
	return err
}

func (r *SQLiteRepo) FindEpisodeByID(id int64) (*domain.Episode, error) {
	query := `SELECT id, podcast_id, title, description, audio_url, published_at, playback_duration, is_played, is_downloaded, local_path FROM episodes WHERE id = ?`
	var e domain.Episode
	err := r.db.QueryRow(query, id).Scan(&e.ID, &e.PodcastID, &e.Title, &e.Description, &e.AudioURL, &e.PublishedAt, &e.PlaybackDuration, &e.IsPlayed, &e.IsDownloaded, &e.LocalPath)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
