// internal/infrastructure/persistence/migrations_test.go
package persistence

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"testing"
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
