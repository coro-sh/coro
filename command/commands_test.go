package command

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	natserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/coro-sh/coro/embedns"
	"github.com/coro-sh/coro/entity"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
	"github.com/coro-sh/coro/server"
	"github.com/coro-sh/coro/sqlite"
	"github.com/coro-sh/coro/testutil"
	"github.com/coro-sh/coro/tkn"
)

const testTimeout = 5 * time.Second

func TestNotifyAccountClaimsUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	h := NewEndToEndHarness(ctx, t)
	acc, err := entity.NewAccount(testutil.RandName(), h.Operator)
	require.NoError(t, err)

	err = h.Commander.NotifyAccountClaimsUpdate(ctx, acc)
	require.NoError(t, err)
}

func TestListStreams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	h := NewEndToEndHarness(ctx, t)

	user, err := entity.NewUser(testutil.RandName(), h.ExistingAccount)
	require.NoError(t, err)

	nc, err := nats.Connect(h.DownstreamNATS.ClientURL(), nats.UserJWTAndSeed(user.JWT(), string(user.NKey().Seed)))
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	stream1, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "test_stream_1",
		Subjects: []string{"test_subject_1.*"},
	})
	require.NoError(t, err)

	stream2, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "test_stream_2",
		Subjects: []string{"test_subject_2.*"},
	})
	require.NoError(t, err)

	got, err := h.Commander.ListStreams(ctx, h.ExistingAccount)
	require.NoError(t, err)

	// assert all fields are equal except for timestamp
	assertEqualStreamInfo := func(want *jetstream.StreamInfo, got *jetstream.StreamInfo) {
		assert.Equal(t, want.Config.Name, got.Config.Name)
		assert.Equal(t, want.Config.Subjects, got.Config.Subjects)
		assert.Equal(t, want.Created, got.Created)
		assert.Equal(t, want.Cluster, got.Cluster)
		assert.Equal(t, want.Mirror, got.Mirror)
		assert.Equal(t, want.Sources, got.Sources)
		assert.Equal(t, want.State, got.State)
	}

	assertEqualStreamInfo(stream1.CachedInfo(), got[0])
	assertEqualStreamInfo(stream2.CachedInfo(), got[1])
}

func TestFetchStreamMessages(t *testing.T) {
	const numMsgsInStream = 100
	const startSeq = uint64(2)
	const batchSize = uint32(98)
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	h := NewEndToEndHarness(ctx, t)

	user, err := entity.NewUser(testutil.RandName(), h.ExistingAccount)
	require.NoError(t, err)

	nc, err := nats.Connect(h.DownstreamNATS.ClientURL(), nats.UserJWTAndSeed(user.JWT(), string(user.NKey().Seed)))
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	streamCfg := jetstream.StreamConfig{
		Name:     "test_stream",
		Subjects: []string{"test_subject.*"},
	}
	_, err = js.CreateOrUpdateStream(ctx, streamCfg)
	require.NoError(t, err)

	for i := 0; i < numMsgsInStream; i++ {
		err = nc.Publish("test_subject.foo", nil)
		require.NoError(t, err)
	}

	msgBatch, err := h.Commander.FetchStreamMessages(ctx, h.ExistingAccount, streamCfg.Name, startSeq, batchSize)
	require.NoError(t, err)
	require.Len(t, msgBatch.Messages, int(batchSize))
	for i, msg := range msgBatch.Messages {
		assert.Equal(t, uint64(i)+startSeq, msg.StreamSequence)
		assert.GreaterOrEqual(t, msg.Timestamp, startTime.Unix())
	}
}

func TestGetMessageContent(t *testing.T) {
	const numMsgsInStream = 3
	const seq = uint64(2)
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	h := NewEndToEndHarness(ctx, t)

	user, err := entity.NewUser(testutil.RandName(), h.ExistingAccount)
	require.NoError(t, err)

	nc, err := nats.Connect(h.DownstreamNATS.ClientURL(), nats.UserJWTAndSeed(user.JWT(), string(user.NKey().Seed)))
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	streamCfg := jetstream.StreamConfig{
		Name:     "test_stream",
		Subjects: []string{"test_subject.*"},
	}
	_, err = js.CreateOrUpdateStream(ctx, streamCfg)
	require.NoError(t, err)

	wantMsgData := []byte("test_msg_0")
	for i := 0; i < numMsgsInStream; i++ {
		msgData := []byte("test_msg_" + strconv.Itoa(i))
		if uint64(i) == seq-1 {
			wantMsgData = msgData
		}
		err = nc.Publish("test_subject.foo", msgData)
		require.NoError(t, err)
	}

	got, err := h.Commander.GetStreamMessageContent(ctx, h.ExistingAccount, streamCfg.Name, seq)
	require.NoError(t, err)
	assert.Equal(t, seq, got.StreamSequence)
	assert.GreaterOrEqual(t, got.Timestamp, startTime.Unix())
	assert.Equal(t, wantMsgData, got.Data)
}

func TestStartConsumer(t *testing.T) {
	const numMsgs = 100
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	h := NewEndToEndHarness(ctx, t)

	user, err := entity.NewUser(testutil.RandName(), h.ExistingAccount)
	require.NoError(t, err)

	nc, err := nats.Connect(h.DownstreamNATS.ClientURL(), nats.UserJWTAndSeed(user.JWT(), string(user.NKey().Seed)))
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	streamCfg := jetstream.StreamConfig{
		Name:     "test_stream",
		Subjects: []string{"test_subject.*"},
	}
	_, err = js.CreateOrUpdateStream(ctx, streamCfg)
	require.NoError(t, err)

	for i := 0; i < numMsgs; i++ {
		err = nc.Publish("test_subject.foo", nil)
		require.NoError(t, err)
	}

	repliesCh := make(chan *commandv1.ReplyMessage, numMsgs)
	consumer, err := h.Commander.ConsumeStream(h.ExistingAccount, streamCfg.Name, 1, func(msg *commandv1.ReplyMessage) {
		repliesCh <- msg
	})
	require.NoError(t, err)

	// empty first reply indicates consumer started
	reply := recvCtx(t, ctx, repliesCh)
	require.Nil(t, reply.Error)
	require.Empty(t, reply.Data)

	for i := 0; i < numMsgs; i++ {
		reply = recvCtx(t, ctx, repliesCh)
		got := &commandv1.StreamConsumerMessage{}
		err = proto.Unmarshal(reply.Data, got)
		require.NoError(t, err)

		wantSeq := uint64(i) + 1
		assert.Equal(t, wantSeq, got.StreamSequence)
		assert.GreaterOrEqual(t, got.Timestamp, startTime.Unix())
		assert.Equal(t, uint64(numMsgs)-wantSeq, got.MessagesPending)
	}

	err = consumer.Stop(ctx)
	require.NoError(t, err)
}

func TestConsumerHeartbeat(t *testing.T) {
	const maxIdleHeartbeat = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	h := NewEndToEndHarness(ctx, t)
	// Override max idle heartbeat for test
	h.Proxy.consumerPool.maxIdleHeartbeat = maxIdleHeartbeat

	user, err := entity.NewUser(testutil.RandName(), h.ExistingAccount)
	require.NoError(t, err)

	nc, err := nats.Connect(h.DownstreamNATS.ClientURL(), nats.UserJWTAndSeed(user.JWT(), string(user.NKey().Seed)))
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	streamCfg := jetstream.StreamConfig{
		Name:     "test_stream",
		Subjects: []string{"test_subject.*"},
	}
	_, err = js.CreateOrUpdateStream(ctx, streamCfg)
	require.NoError(t, err)

	gotRepliesCh := make(chan *commandv1.ReplyMessage, 1)
	consumer, err := h.Commander.ConsumeStream(h.ExistingAccount, streamCfg.Name, 1, func(msg *commandv1.ReplyMessage) {
		gotRepliesCh <- msg
	})
	require.NoError(t, err)

	got := recvCtx(t, ctx, gotRepliesCh)
	require.Empty(t, got.Data) // empty first reply indicates consumer started

	// Send heartbeats over a total duration which exceeds the max idle heartbeat allowed
	for i := 0; i < 3; i++ {
		time.Sleep(maxIdleHeartbeat / 2)
		err = consumer.SendHeartbeat(ctx)
		require.NoError(t, err)
		// Check that the consumer still exists
		_, exists := h.Proxy.consumerPool.consumers.Get(consumer.ID())
		require.True(t, exists)
	}

	// Don't send a heartbeat but wait for next message until consumer stopped due
	// to no idle heartbeat received.
	got = recvCtx(t, ctx, gotRepliesCh)
	gotErr := got.GetError()
	assert.Contains(t, gotErr, "consumer idle heartbeat timeout exceeded")

	_, exists := h.Proxy.consumerPool.consumers.Get(consumer.ID())
	assert.False(t, exists)

	err = consumer.Stop(ctx)
	require.NoError(t, err)
}

type EndToEndHarness struct {
	// Downstream
	Operator        *entity.Operator
	ExistingAccount *entity.Account
	DownstreamNATS  *natserver.Server
	// Command Broker
	BrokerNATS   *natserver.Server
	BrokerServer *server.Server
	Commander    *Commander
	Proxy        *Proxy
}

func NewEndToEndHarness(ctx context.Context, t *testing.T) *EndToEndHarness {
	t.Helper()
	db := sqlite.NewTestDB(t)
	repo := sqlite.NewEntityRepository(db)
	store := entity.NewStore(sqlite.NewTxer(db), repo)
	tknIss := tkn.NewOperatorIssuer(sqlite.NewOperatorTokenReadWriter(db), tkn.OperatorTokenTypeProxy)

	// Setup downstream nats

	op, sysAcc, sysUser := setupEntities(ctx, t, store)
	downstreamNS := startDownstreamNATS(t, op, sysAcc)
	t.Cleanup(downstreamNS.Shutdown)

	existingAcc, err := entity.NewAccount("existing_account", op)
	require.NoError(t, err)
	saveAccountToNATS(t, downstreamNS, sysUser, existingAcc)

	// Generate proxy token

	token, err := tknIss.Generate(ctx, op.ID)
	require.NoError(t, err)

	// Setup broker WebSocket server

	brokerOp, brokerSysAcc, brokerSysUsr := setupEntities(ctx, t, store)
	brokerNS := startEmbeddedNATS(t, brokerOp, brokerSysAcc, "broker_nats")
	t.Cleanup(brokerNS.Shutdown)

	brokerHandler, err := NewBrokerWebSocketHandler(brokerSysUsr, brokerNS, tknIss, store)
	require.NoError(t, err)

	srv, err := server.NewServer(testutil.GetFreePort(t))
	require.NoError(t, err)
	srv.Register("", brokerHandler)

	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, srv.Stop(ctx))
	})

	// Setup proxy

	brokerAddr := srv.WebsSocketAddress() + "/broker"
	proxy, err := NewProxy(ctx, downstreamNS.ClientURL(), brokerAddr, token)
	require.NoError(t, err)
	go proxy.Start(ctx)
	t.Cleanup(func() {
		assert.NoError(t, proxy.Stop())
	})
	time.Sleep(100 * time.Millisecond)

	// Check proxy bridge active

	commander, err := NewCommander(brokerNS.ClientURL(), brokerSysUsr)
	require.NoError(t, err)

	status, err := commander.Ping(ctx, op.ID)
	require.NoError(t, err)
	require.True(t, status.Connected)

	return &EndToEndHarness{
		Operator:        op,
		ExistingAccount: existingAcc,
		DownstreamNATS:  downstreamNS,
		BrokerNATS:      brokerNS,
		BrokerServer:    srv,
		Commander:       commander,
		Proxy:           proxy,
	}
}

func setupEntities(
	ctx context.Context,
	t *testing.T,
	store *entity.Store,
) (*entity.Operator, *entity.Account, *entity.User) {
	ns := entity.NewNamespace(testutil.RandName())
	require.NoError(t, store.CreateNamespace(ctx, ns))

	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
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
	t.Helper()
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

func startDownstreamNATS(t *testing.T, op *entity.Operator, sysAcc *entity.Account) *natserver.Server {
	t.Helper()
	cfgContent, err := entity.NewDirResolverConfig(op, sysAcc, t.TempDir())
	require.NoError(t, err)

	cfgFile, err := os.CreateTemp(t.TempDir(), "")
	require.NoError(t, err)

	err = os.WriteFile(cfgFile.Name(), []byte(cfgContent), 0666)
	require.NoError(t, err)

	opts, err := natserver.ProcessConfigFile(cfgFile.Name())
	require.NoError(t, err)

	opts.Port = natserver.RANDOM_PORT
	opts.JetStream = true

	ns, err := natserver.NewServer(opts)
	require.NoError(t, err)
	ns.Start()

	require.True(t, ns.ReadyForConnections(testTimeout))
	return ns
}

func saveAccountToNATS(t *testing.T, ns *natserver.Server, sysUsr *entity.User, acc *entity.Account) {
	t.Helper()
	sysNC, err := nats.Connect(ns.ClientURL(), nats.UserJWTAndSeed(sysUsr.JWT(), string(sysUsr.NKey().Seed)))
	require.NoError(t, err)
	defer sysNC.Close()

	accData, err := acc.Data()
	require.NoError(t, err)

	subject := fmt.Sprintf("$SYS.REQ.ACCOUNT.%s.CLAIMS.UPDATE", accData.PublicKey)
	res, err := sysNC.RequestWithContext(t.Context(), subject, []byte(accData.JWT))
	require.NoError(t, err)
	require.True(t, strings.Contains(string(res.Data), "jwt updated"))
}
