package proxy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	natserver "github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/broker"
	"github.com/coro-sh/coro/embedns"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/internal/testutil"
	"github.com/coro-sh/coro/server"
	"github.com/coro-sh/coro/tkn"
)

const testTimeout = 5 * time.Second

func TestProxy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	store := entity.NewStore(new(testutil.FakeTxer), entity.NewFakeEntityRepository(t))

	// Setup downstream nats

	op, sysAcc, _ := setupEntities(ctx, t, store)
	downstreamNatsName := "downstream_nats"
	downstreamNats := startEmbeddedNATS(t, op, sysAcc, downstreamNatsName)
	defer downstreamNats.Shutdown()

	// Generate proxy token

	tknIss := tkn.NewOperatorIssuer(tkn.NewFakeOperatorTokenReadWriter(t), tkn.OperatorTokenTypeProxy)
	token, err := tknIss.Generate(ctx, op.ID)
	require.NoError(t, err)

	// Setup broker WebSocket server

	brokerOp, brokerSysAcc, brokerSysUsr := setupEntities(ctx, t, store)
	brokerNats := startEmbeddedNATS(t, brokerOp, brokerSysAcc, "broker_nats")
	defer brokerNats.Shutdown()

	brokerHandler, err := broker.NewWebSocketHandler(brokerSysUsr, brokerNats, tknIss, store)
	require.NoError(t, err)

	srv, err := server.NewServer(testutil.GetFreePort(t))
	require.NoError(t, err)
	srv.Register(brokerHandler)

	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)

	// Setup proxy

	brokerAddr := srv.WebsSocketAddress() + "/api/v1/broker"
	proxy, err := Dial(ctx, downstreamNats.ClientURL(), brokerAddr, token)
	require.NoError(t, err)
	go proxy.Start(ctx)
	defer proxy.Stop()
	time.Sleep(100 * time.Millisecond)

	// Check proxy bridge active

	pub, err := broker.DialPublisher(brokerNats.ClientURL(), brokerSysUsr)
	require.NoError(t, err)

	// Normally would publish notifications like account updates, but in this
	// test we just ping the downstream server instead.
	gotReply, err := pub.Publish(ctx, op.ID, "$SYS.REQ.SERVER.PING", nil)
	require.NoError(t, err)

	type pingReply struct {
		Server struct {
			Name string `json:"name"`
		} `json:"server"`
	}

	var gotReplyData pingReply
	err = json.Unmarshal(gotReply, &gotReplyData)
	require.NoError(t, err)
	assert.Equal(t, downstreamNatsName, gotReplyData.Server.Name)
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

func startEmbeddedNATS(t *testing.T, op *entity.Operator, sysAcc *entity.Account, name string) *natserver.Server {
	ns, err := embedns.NewEmbeddedNATS(embedns.EmbeddedNATSConfig{
		NodeName: name,
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
