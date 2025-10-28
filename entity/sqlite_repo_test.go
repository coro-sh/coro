package entity_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/sqlite"
	"github.com/coro-sh/coro/sqlite/migrations"
)

func NewSQLiteRepo(t *testing.T) entity.Repository {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	db, err := sqlite.Open(ctx, sqlite.WithInMemory())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})
	err = sqlite.MigrateDatabase(db, migrations.FS)
	require.NoError(t, err)
	return sqlite.NewEntityRepository(db)
}
