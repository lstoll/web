package sqlkv

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const (
	// DefaultTableName is the default table name for the KV store
	DefaultTableName = "web_sessions"
)

// Common SQL queries, using placeholders that will be replaced with the proper parameter style
const (
	getQueryTemplate = `SELECT data FROM %s WHERE id = ? AND expires_at > CURRENT_TIMESTAMP`
	getQuerySQLite   = `SELECT data FROM %s WHERE id = ? AND datetime(expires_at) > datetime('now')`

	setQueryTemplate    = `INSERT INTO %s (id, data, expires_at) VALUES (?, ?, ?) %s`
	deleteQueryTemplate = `DELETE FROM %s WHERE id = ?`
	gcQueryTemplate     = `DELETE FROM %s WHERE expires_at < CURRENT_TIMESTAMP`
	gcQuerySQLite       = `DELETE FROM %s WHERE datetime(expires_at) < datetime('now')`

	// Dialects handle upsert differently
	mysqlUpsert    = `ON DUPLICATE KEY UPDATE data = VALUES(data), expires_at = VALUES(expires_at)`
	postgresUpsert = `ON CONFLICT(id) DO UPDATE SET data = EXCLUDED.data, expires_at = EXCLUDED.expires_at`
	sqliteUpsert   = `ON CONFLICT(id) DO UPDATE SET data = excluded.data, expires_at = excluded.expires_at`
)

// Dialect represents a specific SQL dialect configuration
type Dialect int

const (
	// Generic is the default dialect (uses standard SQL as much as possible)
	Generic Dialect = iota
	// MySQL dialect
	MySQL
	// PostgreSQL dialect
	PostgreSQL
	// SQLite dialect
	SQLite
)

// SqlKV implements the session.SqlKV interface using database/sql
type SqlKV struct {
	db *sql.DB

	getQuery    string
	setQuery    string
	deleteQuery string
	gcQuery     string

	dialect   Dialect
	tableName string
}

// Opts contains options for configuring the KV store
type Opts struct {
	// TableName is the name of the table to use for the KV store (defaults to "web_sessions")
	TableName string
	// Dialect specifies which SQL dialect to use (defaults to Generic)
	Dialect Dialect
}

// New creates a new KV store backed by database/sql
func New(db *sql.DB, opts *Opts) *SqlKV {
	tableName := DefaultTableName
	dialect := Generic

	if opts != nil {
		if opts.TableName != "" {
			tableName = opts.TableName
		}
		dialect = opts.Dialect
	}

	kv := &SqlKV{
		db:        db,
		dialect:   dialect,
		tableName: tableName,
	}

	// Prepare queries based on dialect
	kv.setupQueries()

	return kv
}

// setupQueries prepares the SQL queries based on the dialect
func (k *SqlKV) setupQueries() {
	var upsertClause string
	var setQueryTmpl string
	var getQueryTmpl string
	var gcQueryTmpl string

	// Configure queries based on dialect
	switch k.dialect {
	case MySQL:
		upsertClause = mysqlUpsert
		setQueryTmpl = setQueryTemplate
		getQueryTmpl = getQueryTemplate
		gcQueryTmpl = gcQueryTemplate
	case PostgreSQL:
		upsertClause = postgresUpsert
		setQueryTmpl = setQueryTemplate
		getQueryTmpl = getQueryTemplate
		gcQueryTmpl = gcQueryTemplate
	case SQLite:
		upsertClause = sqliteUpsert
		setQueryTmpl = setQueryTemplate
		getQueryTmpl = getQuerySQLite
		gcQueryTmpl = gcQuerySQLite
	default: // Generic
		// Use the most widely supported method: try INSERT, on conflict do UPDATE
		upsertClause = sqliteUpsert // SQLite syntax is fairly portable
		setQueryTmpl = setQueryTemplate
		getQueryTmpl = getQueryTemplate
		gcQueryTmpl = gcQueryTemplate
	}

	// Prepare the queries
	k.getQuery = fmt.Sprintf(getQueryTmpl, k.tableName)
	k.setQuery = fmt.Sprintf(setQueryTmpl, k.tableName, upsertClause)
	k.deleteQuery = fmt.Sprintf(deleteQueryTemplate, k.tableName)
	k.gcQuery = fmt.Sprintf(gcQueryTmpl, k.tableName)

	// Convert placeholder style if needed
	if k.dialect == PostgreSQL {
		k.getQuery = convertPlaceholders(k.getQuery)
		k.setQuery = convertPlaceholders(k.setQuery)
		k.deleteQuery = convertPlaceholders(k.deleteQuery)
		k.gcQuery = convertPlaceholders(k.gcQuery)
	}
}

// convertPlaceholders converts ? placeholders to $1, $2, etc. for PostgreSQL
func convertPlaceholders(query string) string {
	result := ""
	count := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			result += fmt.Sprintf("$%d", count)
			count++
		} else {
			result += string(query[i])
		}
	}
	return result
}

// Get retrieves a value by key, checking expiration
func (k *SqlKV) Get(ctx context.Context, key string) (_ []byte, found bool, _ error) {
	var data []byte
	err := k.db.QueryRowContext(ctx, k.getQuery, key).Scan(&data)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("getting %s: %w", key, err)
	}

	return data, true, nil
}

// Set stores a key with a given value and expiration time, creating or updating as needed
func (k *SqlKV) Set(ctx context.Context, key string, expiresAt time.Time, value []byte) error {
	var err error

	// Special handling for SQLite timestamp format
	if k.dialect == SQLite {
		// Format as RFC3339/ISO8601 for SQLite compatibility, ensuring UTC timezone
		_, err = k.db.ExecContext(ctx, k.setQuery, key, value, expiresAt.UTC().Format(time.RFC3339))
	} else {
		// For other databases, let driver handle time.Time
		_, err = k.db.ExecContext(ctx, k.setQuery, key, value, expiresAt)
	}

	if err != nil {
		return fmt.Errorf("setting %s: %w", key, err)
	}
	return nil
}

// Delete removes a key from the store
func (k *SqlKV) Delete(ctx context.Context, key string) error {
	_, err := k.db.ExecContext(ctx, k.deleteQuery, key)
	if err != nil {
		return fmt.Errorf("deleting %s: %w", key, err)
	}
	return nil
}

// GC performs garbage collection, removing expired keys
func (k *SqlKV) GC(ctx context.Context) (deleted int, _ error) {
	result, err := k.db.ExecContext(ctx, k.gcQuery)
	if err != nil {
		return 0, fmt.Errorf("gc: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("getting affected rows count: %w", err)
	}

	return int(rowsAffected), nil
}

// RunGC starts a background goroutine that performs garbage collection at regular intervals
func (k *SqlKV) RunGC(ctx context.Context, interval time.Duration, logger *slog.Logger) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				if logger != nil {
					logger.InfoContext(ctx, "Garbage collection stopped", "reason", ctx.Err())
				}
				return
			case <-ticker.C:
				deleted, err := k.GC(ctx)
				if err != nil {
					if logger != nil {
						logger.ErrorContext(ctx, "Garbage collection failed", "error", err)
					}
				} else if logger != nil {
					logger.InfoContext(ctx, "Garbage collection successful", "deleted_rows", deleted)
				}
			}
		}
	}()
}

// CreateTable creates the sessions table if it doesn't exist
func (k *SqlKV) CreateTable(ctx context.Context) error {
	var (
		query      string
		indexQuery string
	)

	switch k.dialect {
	case MySQL:
		query = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(255) PRIMARY KEY,
			data BLOB NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			INDEX (expires_at)
		)`, k.tableName)
	case PostgreSQL:
		query = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			data BYTEA NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL
		)`, k.tableName)
		// Create index in a separate statement
		indexQuery = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_expires_at_idx ON %s (expires_at)`,
			k.tableName, k.tableName)
	case SQLite:
		query = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			expires_at TEXT NOT NULL
		)`, k.tableName)
		// Create index in a separate statement
		indexQuery = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_expires_at_idx ON %s (expires_at)`,
			k.tableName, k.tableName)
	default:
		// Generic CREATE TABLE that should work on most systems
		query = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			expires_at TIMESTAMP NOT NULL
		)`, k.tableName)
		// Create index in a separate statement
		indexQuery = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_expires_at_idx ON %s (expires_at)`,
			k.tableName, k.tableName)
	}

	_, err := k.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("creating table: %w", err)
	}
	if indexQuery != "" {
		_, err := k.db.ExecContext(ctx, indexQuery)
		if err != nil {
			return fmt.Errorf("creating index: %w", err)
		}
	}

	return nil
}
