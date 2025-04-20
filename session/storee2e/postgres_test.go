package storee2e

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/lstoll/web/session/kvtest"
	"github.com/lstoll/web/session/sqlkv"
)

// TestKV_PostgreSQL tests the compliance of the PostgreSQL implementation with database/sql
func TestKV_PostgreSQL(t *testing.T) {
	// Skip if no PostgreSQL URL is provided
	pgURL := os.Getenv("WEB_TEST_POSTGRESQL_URL")
	if pgURL == "" {
		t.Skip("WEB_TEST_POSTGRESQL_URL environment variable not set, skipping PostgreSQL (pgx) tests")
	}

	// Create a new connection pool
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	t.Cleanup(pool.Close)

	db := stdlib.OpenDBFromPool(pool)

	// Create KV store with PostgreSQL dialect
	kv := sqlkv.New(db, &sqlkv.Opts{
		Dialect: sqlkv.PostgreSQL,
	})

	// Create the table
	if err := kv.CreateTable(context.Background()); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Setup cleanup for test
	clearFunc := func() {
		_, err := db.Exec("DELETE FROM " + sqlkv.DefaultTableName)
		if err != nil {
			t.Fatalf("Failed to clear PostgreSQL table: %v", err)
		}
	}
	t.Cleanup(clearFunc)

	// Run the compliance tests
	kvtest.RunComplianceTest(t, kv, clearFunc)
}
