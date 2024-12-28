package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	natserver "github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/embedns"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/internal/testutil"
	"github.com/coro-sh/coro/server"
	"github.com/coro-sh/coro/tkn"
)

const testTimeout = 5 * time.Second

func TestBroker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	store := entity.NewStore(new(testutil.FakeTxer), entity.NewFakeEntityRepository(t))
	op, sysAcc, sysUsr := setupEntities(ctx, t, store)
	tknIss := tkn.NewOperatorIssuer(tkn.NewFakeOperatorTokenReadWriter(t), tkn.OperatorTokenTypeProxy)
	token, err := tknIss.Generate(ctx, op.ID)
	require.NoError(t, err)

	brokerNats := startEmbeddedNATS(t, op, sysAcc)
	defer brokerNats.Shutdown()

	brokerHandler, err := NewWebSocketHandler(sysUsr, brokerNats, tknIss, store)
	require.NoError(t, err)

	srv, err := server.NewServer(testutil.GetFreePort(t))
	require.NoError(t, err)
	srv.Register(brokerHandler)

	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)

	type payload struct {
		Data string `json:"data"`
	}

	wantSubject := "foo"
	wantData, err := json.Marshal(payload{Data: "Hello"})
	require.NoError(t, err)
	wantReply, err := json.Marshal(payload{Data: "World"})
	require.NoError(t, err)

	subbed := make(chan struct{})
	stopSub := make(chan struct{})

	wg := new(sync.WaitGroup)
	go func() {
		wg.Add(1)
		defer wg.Done()

		wsURL := fmt.Sprintf("%s/api%s/broker", srv.WebsSocketAddress(), versionPath)
		sub, err := DialSubscriber(ctx, wsURL, token)
		require.NoError(t, err)
		defer sub.Unsubscribe()

		sub.Subscribe(ctx, func(msg Message[[]byte]) ([]byte, error) {
			assert.Equal(t, wantData, msg.Data)
			assert.Equal(t, wantSubject, msg.Subject)
			return wantReply, nil
		})
		close(subbed)

		recvCtx(t, ctx, stopSub)
	}()

	pub, err := DialPublisher(brokerNats.ClientURL(), sysUsr)
	require.NoError(t, err)

	recvCtx(t, ctx, subbed)

	status, err := pub.Ping(ctx, op.ID)
	require.NoError(t, err)
	assert.True(t, status.Connected)
	assert.Positive(t, *status.ConnectTime)

	gotReply, err := pub.Publish(ctx, op.ID, wantSubject, wantData)
	require.NoError(t, err)
	assert.Equal(t, wantReply, gotReply)

	close(stopSub)
	wg.Wait()
}

func setupEntities(
	ctx context.Context,
	t *testing.T,
	store *entity.Store,
) (*entity.Operator, *entity.Account, *entity.User) {
	op, err := entity.NewOperator(testutil.RandName(), entity.NewID[entity.NamespaceID]())
	require.NoError(t, err)
	sysAcc, sysUsr, err := op.SetNewSystemAccountAndUser()
	require.NoError(t, err)

	err = store.CreateOperator(ctx, op)
	require.NoError(t, err)
	err = store.CreateAccount(ctx, sysAcc)
	require.NoError(t, err)
	err = store.CreateUser(ctx, sysUsr)
	require.NoError(t, err)
	return op, sysAcc, sysUsr
}

func recvCtx[T any](t *testing.T, ctx context.Context, ch <-chan T) T {
	select {
	case got := <-ch:
		return got
	case <-ctx.Done():
		require.Fail(t, "subscriber timeout")
	}
	return *new(T)
}

func startEmbeddedNATS(t *testing.T, op *entity.Operator, sysAcc *entity.Account) *natserver.Server {
	ns, err := embedns.NewEmbeddedNATS(embedns.EmbeddedNATSConfig{
		Resolver: embedns.ResolverConfig{
			Operator:      op,
			SystemAccount: sysAcc,
		},
	})
	require.NoError(t, err)
	ns.Start()

	ok := ns.ReadyForConnections(testTimeout)
	require.True(t, ok, "nats unhealthy")
	return ns
}
