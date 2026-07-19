// internal/infrastructure/persistence/migrations_test.go
package persistence

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrations(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify podcasts table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='podcasts'").Scan(&tableName)
	if err != nil {
		t.Errorf("podcasts table not created: %v", err)
	}

	// Verify episodes table exists
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='episodes'").Scan(&tableName)
	if err != nil {
		t.Errorf("episodes table not created: %v", err)
	}
}

func TestMigrations_createsSchemaMigrationsTable(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	// schema_migrations table should exist
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&tableName)
	if err != nil {
		t.Errorf("schema_migrations table not created: %v", err)
	}
}

func TestMigrations_recordsVersion1(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	// Version 1 should be recorded
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected version 1 recorded, got %d", count)
	}
}

func TestMigrations_idempotent(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Run migrations twice
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	// Version 1 should be recorded exactly once
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected version 1 recorded once, got %d", count)
	}
}

func TestMigrations_existingDB_backwardCompatibility(t *testing.T) {
	// Create a DB with the old schema (no schema_migrations table)
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Simulate old schema creation
	if _, err := db.Exec(`
		CREATE TABLE podcasts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			feed_url TEXT NOT NULL UNIQUE
		)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE episodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			podcast_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			audio_url TEXT NOT NULL,
			FOREIGN KEY (podcast_id) REFERENCES podcasts(id)
		)
	`); err != nil {
		t.Fatal(err)
	}

	// Now run migrations - should not fail and should mark version 1 as applied
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations on existing DB failed: %v", err)
	}

	// Version 1 should be recorded
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected version 1 recorded, got %d", count)
	}

	// Original tables should still exist
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='podcasts'").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Error("podcasts table missing after migration")
	}
}

func TestMigrations_futureMigration(t *testing.T) {
	// Save original migrations and restore after test
	originalMigrations := migrations
	defer func() { migrations = originalMigrations }()

	// Add a test migration
	migrations = append(migrations, migration{
		version: 999,
		name:    "test_migration",
		up: []string{
			`CREATE TABLE test_migration_table (id INTEGER PRIMARY KEY)`,
		},
	})

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	// Both versions should be recorded
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 migrations recorded, got %d", count)
	}

	// Test table should exist
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_migration_table'").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Error("test_migration_table not created")
	}
}

func TestMigrations_rollbackOnFailure(t *testing.T) {
	// Save original migrations and restore after test
	originalMigrations := migrations
	defer func() { migrations = originalMigrations }()

	// Add a migration that will fail (invalid SQL)
	migrations = append(migrations, migration{
		version: 1000,
		name:    "failing_migration",
		up: []string{
			`INVALID SQL STATEMENT THAT WILL FAIL`,
		},
	})

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Migration should fail
	if err := RunMigrations(db); err == nil {
		t.Fatal("expected migration to fail")
	}

	// Version 1000 should NOT be recorded (rolled back)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1000").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("version 1000 should not be recorded after rollback, got %d", count)
	}

	// Version 1 should still be recorded
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("version 1 should still be recorded, got %d", count)
	}
}
