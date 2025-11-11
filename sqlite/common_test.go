package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/sqlite/migrations"
)

func setupTestDB(t *testing.T) *sql.DB {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	db, err := Open(ctx, WithInMemory())
	require.NoError(t, err)

	err = MigrateDatabase(db, migrations.FS)
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})

	return db
}
