package command

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/server"
	"github.com/coro-sh/coro/testutil"
	"github.com/coro-sh/coro/tkn"
)

func TestHTTPHandler_GenerateToken(t *testing.T) {
	ctx := context.Background()

	tknIssuer := tkn.NewOperatorIssuer(tkn.NewFakeOperatorTokenReadWriter(t), tkn.OperatorTokenTypeProxy)
	entityStore := entity.NewStore(new(testutil.FakeTxer), entity.NewFakeEntityRepository(t))

	srv, err := server.NewServer(testutil.GetFreePort(t), server.WithMiddleware(
		entity.NamespaceContextMiddleware(),
	))
	require.NoError(t, err)
	srv.Register("", NewProxyHTTPHandler(tknIssuer, entityStore, new(pingerStub)))
	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)

	op, err := entity.NewOperator(testutil.RandName(), entity.NewID[entity.NamespaceID]())
	require.NoError(t, err)
	err = entityStore.CreateOperator(ctx, op)
	require.NoError(t, err)

	url := fmt.Sprintf("%s/namespaces/%s/operators/%s/proxy/token", srv.Address(), op.NamespaceID, op.ID)

	res := testutil.Post[server.Response[GenerateProxyTokenResponse]](t, url, nil)
	got := res.Data

	opID, err := tknIssuer.Verify(ctx, got.Token)
	require.NoError(t, err)
	assert.Equal(t, op.ID, opID)
}

func TestHTTPHandler_GetStatus(t *testing.T) {
	ctx := context.Background()

	tknIssuer := tkn.NewOperatorIssuer(tkn.NewFakeOperatorTokenReadWriter(t), tkn.OperatorTokenTypeProxy)
	entityStore := entity.NewStore(new(testutil.FakeTxer), entity.NewFakeEntityRepository(t))

	srv, err := server.NewServer(testutil.GetFreePort(t), server.WithMiddleware(
		entity.NamespaceContextMiddleware(),
	))
	require.NoError(t, err)
	srv.Register("", NewProxyHTTPHandler(tknIssuer, entityStore, new(pingerStub)))
	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)

	op, err := entity.NewOperator(testutil.RandName(), entity.NewID[entity.NamespaceID]())
	require.NoError(t, err)
	err = entityStore.CreateOperator(ctx, op)
	require.NoError(t, err)

	url := fmt.Sprintf("%s/namespaces/%s/operators/%s/proxy/status", srv.Address(), op.NamespaceID, op.ID)

	res := testutil.Get[server.Response[GetProxyStatusResponse]](t, url)
	got := res.Data

	assert.True(t, got.Connected)
}

type pingerStub struct{}

func (p *pingerStub) Ping(_ context.Context, _ entity.OperatorID) (entity.OperatorNATSStatus, error) {
	connectTime := time.Now().Unix()
	return entity.OperatorNATSStatus{
		Connected:   true,
		ConnectTime: &connectTime,
	}, nil
}
