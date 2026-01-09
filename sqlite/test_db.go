package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/joshjon/kit/encrypt"
	"github.com/joshjon/kit/sqlitedb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/sqlite/migrations"
	"github.com/joshjon/kit/testutil"
)

func NewTestEntityStore(t *testing.T) *entity.Store {
	t.Helper()
	db := NewTestDB(t)
	repo := NewEntityRepository(db)

	enc, err := encrypt.NewAES([]byte(testutil.RandString(32)))
	require.NoError(t, err)
	return entity.NewStore(repo, entity.WithEncryption(enc))
}

func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	db, err := sqlitedb.Open(ctx, sqlitedb.WithInMemory())
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})

	err = sqlitedb.Migrate(db, migrations.FS)
	require.NoError(t, err)
	return db
}
