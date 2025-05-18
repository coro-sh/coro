package nscmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/internal/testutil"
	brokerv1 "github.com/coro-sh/coro/proto/gen/broker/v1"
	"github.com/coro-sh/coro/server"
	"github.com/coro-sh/coro/tkn"
)

func TestWebsocketForwardsCommandsAndReplies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	store := entity.NewStore(new(testutil.FakeTxer), entity.NewFakeEntityRepository(t))
	op, sysAcc, sysUsr := setupEntities(ctx, t, store)
	tknIss := tkn.NewOperatorIssuer(tkn.NewFakeOperatorTokenReadWriter(t), tkn.OperatorTokenTypeProxy)
	token, err := tknIss.Generate(ctx, op.ID)
	require.NoError(t, err)

	acc, err := entity.NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	accData, err := acc.Data()
	require.NoError(t, err)

	brokerNats := startEmbeddedNATS(t, op, sysAcc, "broker_nats")
	defer brokerNats.Shutdown()

	brokerHandler, err := NewBrokerWebSocketHandler(sysUsr, brokerNats, tknIss, store)
	require.NoError(t, err)

	srv, err := server.NewServer(testutil.GetFreePort(t))
	require.NoError(t, err)
	srv.Register(brokerHandler)

	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)

	subbed := make(chan struct{})
	stopSub := make(chan struct{})

	wg := new(sync.WaitGroup)
	go func() {
		wg.Add(1)
		defer wg.Done()

		wsURL := fmt.Sprintf("%s/api%s/broker", srv.WebsSocketAddress(), versionPath)
		sub, err := NewCommandSubscriber(ctx, wsURL, token)
		require.NoError(t, err)
		defer sub.Unsubscribe()

		sub.Subscribe(ctx, func(msg *brokerv1.PublishMessage, replier SubscriptionReplier) error {
			gotReq := msg.GetRequest()
			assert.NotEmpty(t, msg.Id)
			assert.NotEmpty(t, msg.OperationReplyInbox)
			assert.Equal(t, accData.JWT, string(gotReq.Data))
			assert.Equal(t, fmt.Sprintf(accClaimsUpdateSubjectFormat, accData.PublicKey), gotReq.Subject)

			wantReply, err := json.Marshal(accountUpdateReplyMessage{
				Data: natsAccountMessageData{
					Code:    200,
					Account: accData.PublicKey,
					Message: "jwt updated",
				},
			})
			require.NoError(t, err)

			return replier(ctx, &brokerv1.ReplyMessage{
				Id:    msg.Id,
				Inbox: msg.OperationReplyInbox,
				Data:  wantReply,
			})
		})
		close(subbed)

		recvCtx(t, ctx, stopSub)
	}()

	commander, err := NewCommander(brokerNats.ClientURL(), sysUsr)
	require.NoError(t, err)

	recvCtx(t, ctx, subbed)

	status, err := commander.Ping(ctx, op.ID)
	require.NoError(t, err)
	assert.True(t, status.Connected)
	assert.Positive(t, *status.ConnectTime)

	err = commander.NotifyAccountClaimsUpdate(ctx, acc)
	require.NoError(t, err)

	close(stopSub)
	wg.Wait()
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
