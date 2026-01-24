package streamapi

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/joshjon/kit/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/joshjon/kit/testutil"

	"github.com/coro-sh/coro/command"
	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/entityapi"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
	"github.com/coro-sh/coro/sqlite"
)

func TestStreamWebSocketHandler_HandleConsume(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	store := sqlite.NewTestEntityStore(t)

	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	err := store.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	require.NoError(t, err)
	err = store.CreateOperator(ctx, op)
	require.NoError(t, err)

	acc, err := entity.NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	err = store.CreateAccount(ctx, acc)
	require.NoError(t, err)

	const numMsgs = 10
	pubMsgCh := make(chan *commandv1.ReplyMessage, numMsgs)
	consumerStarter := newStreamerStub(nil, pubMsgCh)

	handler := NewStreamWebSocketHandler(store, consumerStarter)

	srv, err := server.NewServer(testutil.GetFreePort(t), server.WithMiddleware(entityapi.NamespaceContextMiddleware()))
	require.NoError(t, err)
	srv.Register("", handler)
	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)
	defer srv.Stop(ctx)

	streamName := testutil.RandName()

	url := fmt.Sprintf("%s/namespaces/%s/accounts/%s/streams/%s/consume", srv.WebsSocketAddress(), acc.NamespaceID, acc.ID, streamName)
	ws, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPClient:   http.DefaultClient,
		Subprotocols: []string{streamWebSocketSubprotocol},
	})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return int64(1) == handler.NumConnections()
	}, 100*time.Millisecond, 5*time.Millisecond)

	// first reply will have empty data to indicate consumer has started
	pubMsgCh <- &commandv1.ReplyMessage{
		Id:    command.NewMessageID().String(),
		Inbox: testutil.RandName(),
		Data:  nil,
	}

	wantConsumerMsgs := make(chan *commandv1.StreamConsumerMessage, numMsgs)
	go func() {
		for i := 0; i < numMsgs; i++ {
			streamSeq := uint64(i + 1)
			cmsg := &commandv1.StreamConsumerMessage{
				StreamSequence:  streamSeq,
				MessagesPending: uint64(numMsgs) - streamSeq,
				Timestamp:       time.Now().Unix(),
			}
			data, err := proto.Marshal(cmsg)
			require.NoError(t, err)

			pubMsgCh <- &commandv1.ReplyMessage{
				Id:    command.NewMessageID().String(),
				Inbox: testutil.RandName(),
				Data:  data,
			}
			wantConsumerMsgs <- cmsg
		}
	}()

	for i := 0; i < numMsgs; i++ {
		var res server.Response[*commandv1.StreamConsumerMessage]
		err = wsjson.Read(ctx, ws, &res)
		require.NoError(t, err)
		want := testutil.AssertReceiveChanContext(t, ctx, wantConsumerMsgs)
		require.True(t, proto.Equal(want, res.Data))
	}

	assert.Len(t, wantConsumerMsgs, 0) // no more expected messages

	err = ws.Close(websocket.StatusNormalClosure, "")
	require.NoError(t, err)

	// wait for websocket closed on the server side
	require.Eventually(t, func() bool {
		return int64(0) == handler.NumConnections()
	}, 100*time.Millisecond, 5*time.Millisecond)
}
