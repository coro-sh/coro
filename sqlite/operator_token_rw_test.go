package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/testutil"
	"github.com/coro-sh/coro/tkn"
)

func TestOperatorTokenReadWriter_ReadWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Setup operator prerequisite
	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	opData := genOperatorData(ns)
	err = repo.CreateOperator(ctx, opData)
	require.NoError(t, err)

	prepo := NewOperatorTokenReadWriter(db)
	wantTkn := testutil.RandString(30)

	err = prepo.Write(ctx, tkn.OperatorTokenTypeProxy, opData.ID, wantTkn)
	require.NoError(t, err)

	got, err := prepo.Read(ctx, tkn.OperatorTokenTypeProxy, opData.ID)
	require.NoError(t, err)
	assert.Equal(t, wantTkn, got)
}
