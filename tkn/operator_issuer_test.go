package tkn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/entity"
)

func TestOperatorIssuer(t *testing.T) {
	ctx := context.Background()
	rw := NewFakeOperatorTokenReadWriter(t)
	issuer := NewOperatorIssuer(rw, OperatorTokenTypeProxy)
	opID := entity.NewID[entity.OperatorID]()

	token, err := issuer.Generate(ctx, opID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	gotHashed, err := rw.Read(ctx, OperatorTokenTypeProxy, opID)
	require.NoError(t, err)
	assert.NotEmpty(t, gotHashed)

	gotOpID, err := issuer.Verify(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, opID, gotOpID)
}
