package storee2e

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/lstoll/web/session/kvtest"
	"github.com/lstoll/web/session/sqlkv"
	_ "github.com/mattn/go-sqlite3" // Import SQLite driver
)

func setupSQLiteDB(t *testing.T) (*sql.DB, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite database: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func TestKV_SQLite(t *testing.T) {
	db, cleanup := setupSQLiteDB(t)
	t.Cleanup(cleanup)

	// Create KV store with SQLite dialect
	kv := sqlkv.New(db, &sqlkv.Opts{
		Dialect: sqlkv.SQLite,
	})

	// Create the table
	if err := kv.CreateTable(context.Background()); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Setup cleanup for test
	clearFunc := func() {
		_, err := db.Exec("DELETE FROM " + sqlkv.DefaultTableName)
		if err != nil {
			t.Fatalf("Failed to clear table: %v", err)
		}
	}
	t.Cleanup(clearFunc)

	// Run the compliance tests
	kvtest.RunComplianceTest(t, kv, clearFunc)
}

func TestKV_SQLite_GC(t *testing.T) {
	db, cleanup := setupSQLiteDB(t)
	defer cleanup()

	// Create KV store
	kv := sqlkv.New(db, &sqlkv.Opts{
		Dialect: sqlkv.SQLite,
	})

	// Create the table
	if err := kv.CreateTable(context.Background()); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	ctx := context.Background()

	// Insert some test data directly
	_, err := db.Exec("INSERT INTO "+sqlkv.DefaultTableName+" (id, data, expires_at) VALUES (?, ?, ?)",
		"expired1", []byte(`{"test":"data1"}`), time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	_, err = db.Exec("INSERT INTO "+sqlkv.DefaultTableName+" (id, data, expires_at) VALUES (?, ?, ?)",
		"expired2", []byte(`{"test":"data2"}`), time.Now().Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	_, err = db.Exec("INSERT INTO "+sqlkv.DefaultTableName+" (id, data, expires_at) VALUES (?, ?, ?)",
		"valid", []byte(`{"test":"data3"}`), time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Run GC
	deleted, err := kv.GC(ctx)
	if err != nil {
		t.Fatalf("GC failed: %v", err)
	}

	// Should have deleted 2 expired keys
	if deleted != 2 {
		t.Errorf("Expected 2 deleted keys, got %d", deleted)
	}

	// Check that expired keys are removed
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM "+sqlkv.DefaultTableName+" WHERE id IN (?, ?)",
		"expired1", "expired2").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count expired keys: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 expired keys, got %d", count)
	}

	// Check that valid key still exists
	var data []byte
	err = db.QueryRow("SELECT data FROM "+sqlkv.DefaultTableName+" WHERE id = ?", "valid").Scan(&data)
	if err != nil {
		t.Fatalf("Failed to get valid key: %v", err)
	}
	if string(data) != `{"test":"data3"}` {
		t.Errorf("Expected valid key data to be preserved, got %s", string(data))
	}
}
