package storee2e

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql" // Import MySQL driver
	"github.com/lstoll/web/session/kvtest"
	"github.com/lstoll/web/session/sqlkv"
)

// TestKV_MySQL tests the compliance of the MySQL implementation
func TestKV_MySQL(t *testing.T) {
	// Skip if no MySQL URL is provided
	mysqlURL := os.Getenv("WEB_TEST_MYSQL_URL")
	if mysqlURL == "" {
		t.Skip("WEB_TEST_MYSQL_URL environment variable not set, skipping MySQL tests")
	}

	// Connect to MySQL
	db, err := sql.Open("mysql", mysqlURL)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Ping to verify connection
	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping MySQL: %v", err)
	}

	// Create KV store with MySQL dialect
	kv := sqlkv.New(db, &sqlkv.Opts{
		Dialect: sqlkv.MySQL,
	})

	// Create the table
	if err := kv.CreateTable(context.Background()); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Setup cleanup for test
	clearFunc := func() {
		_, err := db.Exec("DELETE FROM " + sqlkv.DefaultTableName)
		if err != nil {
			t.Fatalf("Failed to clear MySQL table: %v", err)
		}
	}
	t.Cleanup(clearFunc)

	// Run the compliance tests
	kvtest.RunComplianceTest(t, kv, clearFunc)
}
