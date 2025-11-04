package tkn_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/sqlite"
	"github.com/coro-sh/coro/testutil"
	"github.com/coro-sh/coro/tkn"
)

func TestOperatorIssuer(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	db := sqlite.NewTestDB(t)

	repo := sqlite.NewEntityRepository(db)
	txer := sqlite.NewTxer(db)
	store := entity.NewStore(txer, repo)

	rw := sqlite.NewOperatorTokenReadWriter(db)
	issuer := tkn.NewOperatorIssuer(rw, tkn.OperatorTokenTypeProxy)

	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	require.NoError(t, store.CreateNamespace(ctx, ns))
	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	require.NoError(t, err)
	require.NoError(t, store.CreateOperator(ctx, op))

	token, err := issuer.Generate(ctx, op.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	gotHashed, err := rw.Read(ctx, tkn.OperatorTokenTypeProxy, op.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, gotHashed)

	gotOpID, err := issuer.Verify(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, op.ID, gotOpID)
}
