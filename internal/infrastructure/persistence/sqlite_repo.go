package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/amurru/gocaster/internal/domain"
	_ "github.com/mattn/go-sqlite3"
	sqlite3 "github.com/mattn/go-sqlite3"
)

type SQLiteRepo struct {
	db *sql.DB
}

// sqlitePragmas is applied to every connection pooled by database/sql. These
// are connection-local settings in SQLite, so each pooled connection must run
// them — applying them once is not enough (issue #5).
//
//   - foreign_keys = ON : the schema's ON DELETE CASCADE clauses are otherwise
//     parsed but never enforced, leaving orphaned episodes/download jobs when a
//     podcast is deleted.
//   - busy_timeout = 5000 : a writer blocks readers; without a busy timeout a
//     contended write fails immediately with "database is locked".
//
// journal_mode = WAL is a database-file-level (not connection-local) setting,
// so it is set once via db.Exec below.
var sqlitePragmas = []string{
	"PRAGMA foreign_keys = ON",
	"PRAGMA busy_timeout = 5000",
}

// pragmaDriverName is the registered driver variant that applies the
// connection-local SQLite pragmas via ConnectHook on every pooled connection.
const pragmaDriverName = "gocaster_sqlite3"

var registerPragmaDriverOnce sync.Once

// registerPragmaDriver registers the sqlite3 driver variant with the pragma
// ConnectHook exactly once; calling it again is a no-op (sql.Register panics
// on duplicate names, which would break tests that create multiple repos).
func registerPragmaDriver() {
	registerPragmaDriverOnce.Do(func() {
		sql.Register(pragmaDriverName, &sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				for _, pragma := range sqlitePragmas {
					if _, err := conn.Exec(pragma, nil); err != nil {
						return fmt.Errorf("apply %q: %w", pragma, err)
					}
				}
				return nil
			},
		})
	})
}

func NewSQLiteRepo(dsn string) (*SQLiteRepo, error) {
	// Register the driver variant whose ConnectHook runs the connection-local
	// pragmas on every new pooled connection. database/sql may open more
	// connections as the pool grows; SQLite pragmas like foreign_keys are
	// per-connection, so a one-shot Exec on the first connection is not enough.
	registerPragmaDriver()

	db, err := sql.Open(pragmaDriverName, dsn)
	if err != nil {
		return nil, err
	}

	// SQLite serializes writes; a single connection avoids lock contention and
	// keeps the per-connection pragmas stable.
	db.SetMaxOpenConns(1)
	// Never recycle the pooled connection. The ConnectHook would re-apply on a
	// new connection anyway, but there's no reason to churn for a local DB.
	db.SetConnMaxLifetime(0)

	// journal_mode is a persistent, database-file-level setting (it survives
	// across opens), but set it here so a fresh/empty DB is configured too. On
	// an in-memory database SQLite keeps "memory" mode, which is fine for tests.
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set journal_mode: %w", err)
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

// execer is the minimal interface both *sql.DB and *sql.Tx satisfy, so the
// private save*/mark*/delete* helpers can run against either (issue #13).
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (r *SQLiteRepo) Save(ctx context.Context, podcast *domain.Podcast) error {
	return savePodcast(ctx, r.db, podcast)
}

// savePodcast inserts a new podcast (when ID == 0) or updates an existing one.
// Takes an execer so it can run against either the pool or a transaction.
func savePodcast(ctx context.Context, ex execer, podcast *domain.Podcast) error {
	if podcast.ID == 0 {
		query := `
			INSERT INTO podcasts (title, feed_url, description, image_url, last_updated)
			VALUES (?, ?, ?, ?, ?)
		`
		result, err := ex.ExecContext(
			ctx,
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
	_, err := ex.ExecContext(
		ctx,
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

func (r *SQLiteRepo) FindAll(ctx context.Context) ([]domain.Podcast, error) {
	query := `SELECT id, title, feed_url, description, image_url, last_updated FROM podcasts ORDER BY title`
	rows, err := r.db.QueryContext(ctx, query)
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

func (r *SQLiteRepo) FindByID(ctx context.Context, id int64) (*domain.Podcast, error) {
	query := `SELECT id, title, feed_url, description, image_url, last_updated FROM podcasts WHERE id = ?`
	var p domain.Podcast
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&p.ID, &p.Title, &p.FeedURL, &p.Description, &p.ImageURL, &p.LastUpdated)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *SQLiteRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM podcasts WHERE id = ?`, id)
	return err
}

func (r *SQLiteRepo) SaveEpisode(ctx context.Context, episode *domain.Episode) error {
	return saveEpisode(ctx, r.db, episode)
}

// saveEpisode inserts an episode. Takes an execer so it can run inside a
// transaction. On success episode.ID reflects the newly assigned row.
func saveEpisode(ctx context.Context, ex execer, episode *domain.Episode) error {
	query := `
		INSERT INTO episodes (podcast_id, title, description, audio_url, published_at, playback_duration, is_played, is_downloaded, local_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := ex.ExecContext(
		ctx,
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

func (r *SQLiteRepo) FindEpisodesByPodcastID(ctx context.Context, id int64) ([]domain.Episode, error) {
	query := `SELECT id, podcast_id, title, description, audio_url, published_at, playback_duration, is_played, is_downloaded, local_path FROM episodes WHERE podcast_id = ? ORDER BY published_at DESC`
	rows, err := r.db.QueryContext(ctx, query, id)
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

func (r *SQLiteRepo) DeleteEpisode(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM episodes WHERE id = ?`, id)
	return err
}

func (r *SQLiteRepo) FindEpisodeByID(ctx context.Context, id int64) (*domain.Episode, error) {
	query := `SELECT id, podcast_id, title, description, audio_url, published_at, playback_duration, is_played, is_downloaded, local_path FROM episodes WHERE id = ?`
	var e domain.Episode
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&e.ID, &e.PodcastID, &e.Title, &e.Description, &e.AudioURL, &e.PublishedAt, &e.PlaybackDuration, &e.IsPlayed, &e.IsDownloaded, &e.LocalPath)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *SQLiteRepo) UpdateEpisodePlaybackState(ctx context.Context, id int64, isPlayed bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE episodes SET is_played = ? WHERE id = ?`, isPlayed, id)
	return err
}

func (r *SQLiteRepo) SaveDownloadJob(ctx context.Context, job *domain.DownloadJob) error {
	query := `
		INSERT INTO downloads (episode_id, status, bytes_downloaded, bytes_total, temp_path, final_path, etag, last_modified, supports_resume, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(
		ctx,
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

func (r *SQLiteRepo) FindDownloadJobByEpisodeID(ctx context.Context, episodeID int64) (*domain.DownloadJob, error) {
	query := `SELECT id, episode_id, status, bytes_downloaded, bytes_total, temp_path, final_path, etag, last_modified, supports_resume, error_message, created_at, updated_at FROM downloads WHERE episode_id = ?`
	var j domain.DownloadJob
	err := r.db.QueryRowContext(ctx, query, episodeID).
		Scan(&j.ID, &j.EpisodeID, &j.Status, &j.BytesDownloaded, &j.BytesTotal, &j.TempPath, &j.FinalPath, &j.ETag, &j.LastModified, &j.SupportsResume, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// FindDownloadJobByID loads a single job by primary key. It replaces the
// previous DownloadService.findJobByID helper, which called FindAllDownloadJobs
// and iterated the whole table on every status update (issue #17 perf bug).
func (r *SQLiteRepo) FindDownloadJobByID(ctx context.Context, id int64) (*domain.DownloadJob, error) {
	query := `SELECT id, episode_id, status, bytes_downloaded, bytes_total, temp_path, final_path, etag, last_modified, supports_resume, error_message, created_at, updated_at FROM downloads WHERE id = ?`
	var j domain.DownloadJob
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&j.ID, &j.EpisodeID, &j.Status, &j.BytesDownloaded, &j.BytesTotal, &j.TempPath, &j.FinalPath, &j.ETag, &j.LastModified, &j.SupportsResume, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *SQLiteRepo) FindAllDownloadJobs(ctx context.Context) ([]domain.DownloadJob, error) {
	query := `SELECT id, episode_id, status, bytes_downloaded, bytes_total, temp_path, final_path, etag, last_modified, supports_resume, error_message, created_at, updated_at FROM downloads ORDER BY updated_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
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
	ctx context.Context,
	id int64,
	status domain.DownloadStatus,
	bytesDownloaded int64,
	bytesTotal int64,
	errorMsg string,
) error {
	query := `UPDATE downloads SET status = ?, bytes_downloaded = ?, bytes_total = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, bytesDownloaded, bytesTotal, errorMsg, id)
	return err
}

// UpdateDownloadJobProgress persists the resume-relevant fields of a job
// (counters, temp/final paths, ETag, Last-Modified, SupportsResume) without
// altering its status. This lets runDownload record that the server honored a
// Range request so a subsequent retry can resume from the partial file.
func (r *SQLiteRepo) UpdateDownloadJobProgress(ctx context.Context, job *domain.DownloadJob) error {
	if job == nil {
		return fmt.Errorf("cannot update progress for nil job")
	}
	query := `UPDATE downloads
		SET bytes_downloaded = ?, bytes_total = ?, temp_path = ?, final_path = ?,
		    etag = ?, last_modified = ?, supports_resume = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`
	_, err := r.db.ExecContext(
		ctx,
		query,
		job.BytesDownloaded,
		job.BytesTotal,
		job.TempPath,
		job.FinalPath,
		job.ETag,
		job.LastModified,
		job.SupportsResume,
		job.ID,
	)
	return err
}

func (r *SQLiteRepo) CountNonFailedJobs(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM downloads WHERE status != 'failed'`
	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

func (r *SQLiteRepo) DeleteDownloadJob(ctx context.Context, id int64) error {
	return deleteDownloadJob(ctx, r.db, id)
}

// deleteDownloadJob removes a download-job row. Takes an execer so it can run
// inside the download-completion transaction.
func deleteDownloadJob(ctx context.Context, ex execer, id int64) error {
	_, err := ex.ExecContext(ctx, `DELETE FROM downloads WHERE id = ?`, id)
	return err
}

func (r *SQLiteRepo) MarkEpisodeDownloaded(ctx context.Context, episodeID int64, localPath string) error {
	return markEpisodeDownloaded(ctx, r.db, episodeID, localPath)
}

// markEpisodeDownloaded flips the downloaded flag and stores the local path.
// Takes an execer so it can run inside the download-completion transaction.
func markEpisodeDownloaded(ctx context.Context, ex execer, episodeID int64, localPath string) error {
	query := `UPDATE episodes SET is_downloaded = 1, local_path = ? WHERE id = ?`
	_, err := ex.ExecContext(ctx, query, localPath, episodeID)
	return err
}

// runInTx runs fn inside a transaction bound to the caller's context (issue
// #13). It commits iff fn returns nil, rolls back on any error, and re-panics
// after rolling back on a panic so the caller's recover still sees it. A nil
// error returned from fn does NOT guarantee commit success — the returned
// error reflects the commit outcome.
func (r *SQLiteRepo) runInTx(ctx context.Context, fn func(*sql.Tx) error) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()
	return fn(tx)
}

// SavePodcastWithEpisodes inserts the podcast and every episode in a single
// transaction (issue #13). All-or-nothing: a mid-loop failure rolls back the
// podcast and any episodes saved so far. On success podcast.ID is set.
func (r *SQLiteRepo) SavePodcastWithEpisodes(ctx context.Context, podcast *domain.Podcast, episodes []domain.Episode) error {
	return r.runInTx(ctx, func(tx *sql.Tx) error {
		if err := savePodcast(ctx, tx, podcast); err != nil {
			return err
		}
		for i := range episodes {
			episodes[i].PodcastID = podcast.ID
			if err := saveEpisode(ctx, tx, &episodes[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// AppendEpisodesAndTouchPodcast inserts the (already-filtered) new episodes and
// updates the podcast's LastUpdated in a single transaction (issue #13). A
// failure mid-batch rolls back both the inserts and the LastUpdated bump so the
// feed is retried in full on the next refresh.
func (r *SQLiteRepo) AppendEpisodesAndTouchPodcast(ctx context.Context, podcast *domain.Podcast, newEpisodes []domain.Episode) error {
	return r.runInTx(ctx, func(tx *sql.Tx) error {
		for i := range newEpisodes {
			newEpisodes[i].PodcastID = podcast.ID
			if err := saveEpisode(ctx, tx, &newEpisodes[i]); err != nil {
				return err
			}
		}
		return savePodcast(ctx, tx, podcast)
	})
}

// CompleteDownload marks an episode downloaded and deletes its completed job
// row in a single transaction (issue #13). The two writes must commit together
// to keep the episode and downloads tables in sync. A job-row delete failure
// now rolls back the mark, leaving the job intact for retry.
func (r *SQLiteRepo) CompleteDownload(ctx context.Context, episodeID int64, localPath string, jobID int64) error {
	return r.runInTx(ctx, func(tx *sql.Tx) error {
		if err := markEpisodeDownloaded(ctx, tx, episodeID, localPath); err != nil {
			return err
		}
		return deleteDownloadJob(ctx, tx, jobID)
	})
}
