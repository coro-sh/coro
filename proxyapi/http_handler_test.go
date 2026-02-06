package proxyapi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/joshjon/kit/ref"
	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/entityapi"
	"github.com/coro-sh/coro/sqlite"
	"github.com/coro-sh/coro/tkn"
)

func TestHTTPHandler_GenerateToken(t *testing.T) {
	ctx := testutil.Context(t)
	db := sqlite.NewTestDB(t)
	repo := sqlite.NewEntityRepository(db)
	store := entity.NewStore(repo)
	tknIss := tkn.NewOperatorIssuer(sqlite.NewOperatorTokenReadWriter(db), tkn.OperatorTokenTypeProxy)

	srv, err := server.NewServer(testutil.GetFreePort(t), server.WithMiddleware(
		entityapi.NamespaceContextMiddleware(),
	))
	require.NoError(t, err)
	srv.Register("", NewProxyHTTPHandler(tknIss, store, new(pingerStub)))
	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)

	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	require.NoError(t, store.CreateNamespace(ctx, ns))

	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	require.NoError(t, err)
	err = store.CreateOperator(ctx, op)
	require.NoError(t, err)

	url := fmt.Sprintf("%s/namespaces/%s/operators/%s/proxy/token", srv.Address(), op.NamespaceID, op.ID)

	res := testutil.Post[server.Response[GenerateProxyTokenResponse]](t, url, nil)
	got := res.Data

	opID, err := tknIss.Verify(ctx, got.Token)
	require.NoError(t, err)
	assert.Equal(t, op.ID, opID)
}

func TestHTTPHandler_GetStatus(t *testing.T) {
	ctx := testutil.Context(t)
	db := sqlite.NewTestDB(t)
	repo := sqlite.NewEntityRepository(db)
	store := entity.NewStore(repo)
	tknIss := tkn.NewOperatorIssuer(sqlite.NewOperatorTokenReadWriter(db), tkn.OperatorTokenTypeProxy)

	srv, err := server.NewServer(testutil.GetFreePort(t), server.WithMiddleware(
		entityapi.NamespaceContextMiddleware(),
	))
	require.NoError(t, err)
	srv.Register("", NewProxyHTTPHandler(tknIss, store, new(pingerStub)))
	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)

	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	require.NoError(t, store.CreateNamespace(ctx, ns))

	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	require.NoError(t, err)
	err = store.CreateOperator(ctx, op)
	require.NoError(t, err)

	url := fmt.Sprintf("%s/namespaces/%s/operators/%s/proxy/status", srv.Address(), op.NamespaceID, op.ID)

	res := testutil.Get[server.Response[GetProxyStatusResponse]](t, url)
	got := res.Data

	assert.True(t, got.Connected)
}

type pingerStub struct{}

func (p *pingerStub) Ping(_ context.Context, _ entity.OperatorID) (entity.OperatorNATSStatus, error) {
	return entity.OperatorNATSStatus{
		Connected:   true,
		ConnectTime: ref.Ptr(time.Now().Unix()),
	}, nil
}
