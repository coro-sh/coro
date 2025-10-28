package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	_ "modernc.org/sqlite"

	"github.com/coro-sh/coro/constants"
)

const (
	healthRetryInterval = time.Second
	healthMaxRetries    = 5
)

type OpenOption func(opts *openOpts)

// WithDir sets the directory for the SQLite database file for opening a
// database connection.
func WithDir(dir string) OpenOption {
	return func(opts *openOpts) {
		opts.dir = dir
	}
}

// WithInMemory enables an in-memory SQLite database.
func WithInMemory() OpenOption {
	return func(opts *openOpts) {
		opts.inMemory = true
	}
}

type openOpts struct {
	dir      string
	inMemory bool
}

func Open(ctx context.Context, opts ...OpenOption) (*sql.DB, error) {
	var o openOpts
	for _, opt := range opts {
		opt(&o)
	}

	var dsn string

	if o.inMemory {
		dsn = "file::memory:?cache=shared"
	} else {
		file := constants.AppName + ".db"
		if o.dir != "" {
			if err := os.MkdirAll(o.dir, 0755); err != nil {
				return nil, fmt.Errorf("create sqlite directory: %w", err)
			}
			file = strings.TrimSuffix(o.dir, "/") + "/" + file
		}
		dsn = "file:" + file + "?_journal_mode=WAL&_busy_timeout=5000"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Set max connections to 1 since sqlite only supports a single writer at a time
	db.SetMaxOpenConns(1)

	if _, err = db.Exec("PRAGMA foreign_keys = on;"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if !o.inMemory {
		// Use WAL mode only for file backed DBs
		if _, err = db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
			return nil, fmt.Errorf("enable WAL mode: %w", err)
		}
	}

	if err = waitHealthy(ctx, db); err != nil {
		return nil, err
	}

	return db, nil
}

func waitHealthy(ctx context.Context, db *sql.DB) error {
	pingFn := func() error {
		pctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		return db.PingContext(pctx)
	}
	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(healthRetryInterval), healthMaxRetries)
	if err := backoff.Retry(pingFn, bo); err != nil {
		return fmt.Errorf("sqlite connection unhealthy: %w", err)
	}
	return nil
}
