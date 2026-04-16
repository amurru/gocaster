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
		result, err := r.db.Exec(
			query,
			podcast.Title,
			podcast.FeedURL,
			podcast.Description,
			podcast.ImageURL,
			podcast.LastUpdated,
		)
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
	_, err := r.db.Exec(
		query,
		podcast.Title,
		podcast.FeedURL,
		podcast.Description,
		podcast.ImageURL,
		podcast.LastUpdated,
		podcast.ID,
	)
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
	err := r.db.QueryRow(query, id).
		Scan(&p.ID, &p.Title, &p.FeedURL, &p.Description, &p.ImageURL, &p.LastUpdated)
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
	result, err := r.db.Exec(
		query,
		episode.PodcastID,
		episode.Title,
		episode.Description,
		episode.AudioURL,
		episode.PublishedAt,
		episode.PlaybackDuration,
		episode.IsPlayed,
		episode.IsDownloaded,
		episode.LocalPath,
	)
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
		err := rows.Scan(
			&e.ID,
			&e.PodcastID,
			&e.Title,
			&e.Description,
			&e.AudioURL,
			&e.PublishedAt,
			&e.PlaybackDuration,
			&e.IsPlayed,
			&e.IsDownloaded,
			&e.LocalPath,
		)
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
	err := r.db.QueryRow(query, id).
		Scan(&e.ID, &e.PodcastID, &e.Title, &e.Description, &e.AudioURL, &e.PublishedAt, &e.PlaybackDuration, &e.IsPlayed, &e.IsDownloaded, &e.LocalPath)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *SQLiteRepo) UpdateEpisodePlaybackState(id int64, isPlayed bool) error {
	_, err := r.db.Exec(`UPDATE episodes SET is_played = ? WHERE id = ?`, isPlayed, id)
	return err
}

func (r *SQLiteRepo) SaveDownloadJob(job *domain.DownloadJob) error {
	query := `
		INSERT INTO downloads (episode_id, status, bytes_downloaded, bytes_total, temp_path, final_path, etag, last_modified, supports_resume, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(
		query,
		job.EpisodeID,
		job.Status,
		job.BytesDownloaded,
		job.BytesTotal,
		job.TempPath,
		job.FinalPath,
		job.ETag,
		job.LastModified,
		job.SupportsResume,
		job.ErrorMessage,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	job.ID = id
	return nil
}

func (r *SQLiteRepo) FindDownloadJobByEpisodeID(episodeID int64) (*domain.DownloadJob, error) {
	query := `SELECT id, episode_id, status, bytes_downloaded, bytes_total, temp_path, final_path, etag, last_modified, supports_resume, error_message, created_at, updated_at FROM downloads WHERE episode_id = ?`
	var j domain.DownloadJob
	err := r.db.QueryRow(query, episodeID).
		Scan(&j.ID, &j.EpisodeID, &j.Status, &j.BytesDownloaded, &j.BytesTotal, &j.TempPath, &j.FinalPath, &j.ETag, &j.LastModified, &j.SupportsResume, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *SQLiteRepo) FindAllDownloadJobs() ([]domain.DownloadJob, error) {
	query := `SELECT id, episode_id, status, bytes_downloaded, bytes_total, temp_path, final_path, etag, last_modified, supports_resume, error_message, created_at, updated_at FROM downloads ORDER BY updated_at DESC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []domain.DownloadJob
	for rows.Next() {
		var j domain.DownloadJob
		err := rows.Scan(
			&j.ID,
			&j.EpisodeID,
			&j.Status,
			&j.BytesDownloaded,
			&j.BytesTotal,
			&j.TempPath,
			&j.FinalPath,
			&j.ETag,
			&j.LastModified,
			&j.SupportsResume,
			&j.ErrorMessage,
			&j.CreatedAt,
			&j.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}

	return jobs, rows.Err()
}

func (r *SQLiteRepo) UpdateDownloadJobStatus(
	id int64,
	status domain.DownloadStatus,
	bytesDownloaded int64,
	bytesTotal int64,
	errorMsg string,
) error {
	query := `UPDATE downloads SET status = ?, bytes_downloaded = ?, bytes_total = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := r.db.Exec(query, status, bytesDownloaded, bytesTotal, errorMsg, id)
	return err
}

func (r *SQLiteRepo) CountNonFailedJobs() (int, error) {
	query := `SELECT COUNT(*) FROM downloads WHERE status != 'failed'`
	var count int
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

func (r *SQLiteRepo) DeleteDownloadJob(id int64) error {
	_, err := r.db.Exec(`DELETE FROM downloads WHERE id = ?`, id)
	return err
}

func (r *SQLiteRepo) MarkEpisodeDownloaded(episodeID int64, localPath string) error {
	query := `UPDATE episodes SET is_downloaded = 1, local_path = ? WHERE id = ?`
	_, err := r.db.Exec(query, localPath, episodeID)
	return err
}
