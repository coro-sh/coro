package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/encrypt"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/sqlite/migrations"
	"github.com/coro-sh/coro/testutil"
)

func NewTestEntityStore(t *testing.T) (*entity.Store, *Txer) {
	t.Helper()
	db := NewTestDB(t)
	repo := NewEntityRepository(db)

	enc, err := encrypt.NewAES(testutil.RandString(32))
	require.NoError(t, err)
	txer := NewTxer(db)
	return entity.NewStore(txer, repo, entity.WithEncryption(enc)), txer
}

func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	db, err := Open(ctx, WithInMemory())
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})

	err = MigrateDatabase(db, migrations.FS)
	require.NoError(t, err)
	return db
}
