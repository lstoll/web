package pgxkv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultTableName = "web_sessions"
)

var (
	_ DBConn = (*pgx.Conn)(nil)
	_ DBConn = (*pgxpool.Pool)(nil)
)

type DBConn interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, optionsAndArgs ...interface{}) pgx.Row
}

const (
	getQueryTemplate    = `SELECT data FROM %s WHERE id = $1 AND expires_at > now()`
	setQueryTemplate    = `INSERT INTO %s (id, data, expires_at) VALUES ($1, $2, $3) ON CONFLICT(id) DO UPDATE SET data=EXCLUDED.data, expires_at=EXCLUDED.expires_at`
	deleteQueryTemplate = `DELETE FROM %s WHERE id = $1`
	gcQueryTemplate     = `DELETE FROM %s WHERE expires_at < now()`
)

type KV struct {
	conn DBConn

	getQuery    string
	setQuery    string
	deleteQuery string
	gcQuery     string
}

type Opts struct {
	TableName string
}

func New(conn DBConn, opts *Opts) *KV {
	tn := DefaultTableName
	if opts != nil && opts.TableName != "" {
		tn = opts.TableName
	}
	return &KV{
		conn: conn,

		getQuery:    fmt.Sprintf(getQueryTemplate, tn),
		setQuery:    fmt.Sprintf(setQueryTemplate, tn),
		deleteQuery: fmt.Sprintf(deleteQueryTemplate, tn),
		gcQuery:     fmt.Sprintf(gcQueryTemplate, tn),
	}
}

func (k *KV) Get(ctx context.Context, key string) (_ []byte, found bool, _ error) {
	var data []byte
	if err := k.conn.QueryRow(ctx, k.getQuery, key).Scan(&data); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("getting %s: %w", key, err)
	}
	return data, true, nil
}

func (k *KV) Set(ctx context.Context, key string, expiresAt time.Time, value []byte) error {
	if _, err := k.conn.Exec(ctx, k.setQuery, key, value, expiresAt); err != nil {
		return fmt.Errorf("setting %s: %w", key, err)
	}
	return nil
}

func (k *KV) Delete(ctx context.Context, key string) error {
	if _, err := k.conn.Exec(ctx, k.deleteQuery, key); err != nil {
		return fmt.Errorf("deleting %s: %w", key, err)
	}

	return nil
}

func (k *KV) GC(ctx context.Context) (deleted int, _ error) {
	res, err := k.conn.Exec(ctx, k.gcQuery)
	if err != nil {
		return 0, fmt.Errorf("gc: %w", err)
	}
	return int(res.RowsAffected()), nil
}

func (k *KV) RunGC(ctx context.Context, interval time.Duration, logger *slog.Logger) {
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
				} else {
					if logger != nil {
						logger.InfoContext(ctx, "Garbage collection successful", "deleted_rows", deleted)
					}
				}
			}
		}
	}()
}
