package command

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/internal/testutil"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
	"github.com/coro-sh/coro/server"
)

func TestStreamWebSocketHandler_HandleConsume(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	op, err := entity.NewOperator(testutil.RandName(), entity.NewID[entity.NamespaceID]())
	require.NoError(t, err)
	acc, err := entity.NewAccount(testutil.RandName(), op)
	store := entity.NewStore(new(testutil.FakeTxer), entity.NewFakeEntityRepository(t))
	err = store.CreateAccount(ctx, acc)
	require.NoError(t, err)

	const numMsgs = 10
	pubMsgCh := make(chan *commandv1.ReplyMessage, numMsgs)
	consumerStarter := newStreamerStub(nil, pubMsgCh)

	handler := NewStreamWebSocketHandler(store, consumerStarter)

	srv, err := server.NewServer(testutil.GetFreePort(t), server.WithMiddleware(entity.NamespaceContextMiddleware()))
	require.NoError(t, err)
	srv.Register(handler)
	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)
	defer srv.Stop(ctx)

	url := fmt.Sprintf("%s/api%s/namespaces/%s/accounts/%s/streams/consume", srv.WebsSocketAddress(), entity.VersionPath, acc.NamespaceID, acc.ID)
	ws, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPClient:   http.DefaultClient,
		Subprotocols: []string{streamWebSocketSubprotocol},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), handler.NumConnections())

	startReq := StartStreamConsumerRequest{
		AccountID: acc.ID.String(),
		Stream:    testutil.RandName(),
	}
	err = wsjson.Write(ctx, ws, startReq)
	require.NoError(t, err)

	wantResps := make(chan []byte, numMsgs)
	go func() {
		for i := 0; i < numMsgs; i++ {
			data := []byte(strconv.Itoa(i))
			wantResps <- data
			pubMsgCh <- &commandv1.ReplyMessage{
				Id:    NewMessageID().String(),
				Inbox: testutil.RandName(),
				Data:  data,
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	for i := 0; i < numMsgs; i++ {
		var res server.Response[[]byte]
		err = wsjson.Read(ctx, ws, &res)
		require.NoError(t, err)
		want := recvCtx(t, ctx, wantResps)
		require.Equal(t, string(want), string(res.Data))
	}

	assert.Len(t, wantResps, 0) // no more expected messages

	err = ws.Close(websocket.StatusNormalClosure, "")
	require.NoError(t, err)

	// wait for websocket closed on the server side
	time.Sleep(5 * time.Millisecond)
	require.Equal(t, int64(0), handler.NumConnections())
}
