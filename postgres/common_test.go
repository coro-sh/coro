package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/postgres/migrations"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	recreateDB(ctx, t)
	pool, err := Dial(ctx, "postgres", "postgres", "localhost:5432", AppDBName)
	require.NoError(t, err)

	err = MigrateDatabase(pool, migrations.FS)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool.Close()
		recreateDB(ctx, t)
	})

	return pool
}

func recreateDB(ctx context.Context, t *testing.T) {
	pool, err := Dial(ctx, "postgres", "postgres", "localhost:5432", "postgres")
	require.NoError(t, err)
	err = pool.Ping(ctx)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DROP DATABASE IF EXISTS "+AppDBName)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE DATABASE "+AppDBName)
	require.NoError(t, err)
	pool.Close()
}
