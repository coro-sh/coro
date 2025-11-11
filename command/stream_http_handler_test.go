package command

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
	"github.com/coro-sh/coro/server"
	"github.com/coro-sh/coro/sqlite"
	"github.com/coro-sh/coro/testutil"
)

func TestStreamHTTPHandler_ListStreams(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	now := time.Now()
	want := []StreamResponse{
		{
			Name:          "stream_1",
			Subjects:      []string{"subject_foo", "subject_bar"},
			MessageCount:  100,
			ConsumerCount: 5,
			CreateTime:    now.Unix(),
		},
		{
			Name:          "stream_2",
			Subjects:      []string{"subject_lorem", "subject_ipsum"},
			MessageCount:  1000,
			ConsumerCount: 50,
			CreateTime:    now.Unix(),
		},
	}

	var streams []*jetstream.StreamInfo
	for _, w := range want {
		streams = append(streams, &jetstream.StreamInfo{
			Config:  jetstream.StreamConfig{Name: w.Name, Subjects: w.Subjects},
			State:   jetstream.StreamState{Msgs: w.MessageCount, Consumers: w.ConsumerCount},
			Created: now,
		})
	}

	srv, entityStore := newStreamHTTPServer(t, streams, nil)

	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	err := entityStore.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	err = entityStore.CreateOperator(ctx, op)
	require.NoError(t, err)

	acc, err := entity.NewAccount(testutil.RandName(), op)
	err = entityStore.CreateAccount(ctx, acc)
	require.NoError(t, err)

	url := fmt.Sprintf("%s/namespaces/%s/accounts/%s/streams", srv.Address(), op.NamespaceID, acc.ID)

	res := testutil.Get[server.ResponseList[StreamResponse]](t, url)
	got := res.Data
	assert.Equal(t, want, got)
	assert.Nil(t, res.NextPageCursor)
}

func TestStreamHTTPHandler_FetchStreamMessages(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	srv, entityStore := newStreamHTTPServer(t, nil, nil)

	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	err := entityStore.CreateNamespace(t.Context(), ns)
	require.NoError(t, err)

	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	require.NoError(t, err)
	err = entityStore.CreateOperator(ctx, op)
	require.NoError(t, err)

	acc, err := entity.NewAccount(testutil.RandName(), op)
	err = entityStore.CreateAccount(ctx, acc)
	require.NoError(t, err)

	url := fmt.Sprintf(
		"%s/namespaces/%s/accounts/%s/streams/fake_stream/messages?start_sequence=1&batch_size=10",
		srv.Address(), op.NamespaceID, acc.ID,
	)

	res := testutil.Get[server.ResponseList[*commandv1.StreamMessage]](t, url)
	got := res.Data
	require.NotEmpty(t, res.Data)
	for _, msg := range got {
		assert.NotZero(t, msg.StreamSequence)
		assert.NotZero(t, msg.Timestamp)
	}
}

func TestStreamHTTPHandler_GetStreamMessageContent(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	srv, entityStore := newStreamHTTPServer(t, nil, nil)

	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	err := entityStore.CreateNamespace(t.Context(), ns)
	require.NoError(t, err)

	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	require.NoError(t, err)
	err = entityStore.CreateOperator(ctx, op)
	require.NoError(t, err)

	acc, err := entity.NewAccount(testutil.RandName(), op)
	err = entityStore.CreateAccount(ctx, acc)
	require.NoError(t, err)

	url := fmt.Sprintf(
		"%s/namespaces/%s/accounts/%s/streams/fake_stream/messages/1",
		srv.Address(), op.NamespaceID, acc.ID,
	)

	res := testutil.Get[server.Response[*commandv1.StreamMessageContent]](t, url)
	got := res.Data
	require.NotEmpty(t, res.Data)
	assert.NotZero(t, got.StreamSequence)
	assert.NotZero(t, got.Timestamp)
	assert.NotEmpty(t, got.Data)
}

func newStreamHTTPServer(t *testing.T, streams []*jetstream.StreamInfo, msgs <-chan *commandv1.ReplyMessage) (*server.Server, *entity.Store) {
	store := sqlite.NewTestEntityStore(t)

	srv, err := server.NewServer(testutil.GetFreePort(t), server.WithMiddleware(entity.NamespaceContextMiddleware()))
	require.NoError(t, err)
	srv.Register("", NewStreamHTTPHandler(store, newStreamerStub(streams, msgs)))

	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, srv.Stop(t.Context()))
	})

	return srv, store
}

type streamerStub struct {
	streams []*jetstream.StreamInfo
	msgs    <-chan *commandv1.ReplyMessage
}

func newStreamerStub(stubStreams []*jetstream.StreamInfo, stubMsgs <-chan *commandv1.ReplyMessage) *streamerStub {
	return &streamerStub{streams: stubStreams, msgs: stubMsgs}
}

func (s *streamerStub) ListStreams(_ context.Context, _ *entity.Account) ([]*jetstream.StreamInfo, error) {
	return s.streams, nil
}

func (s *streamerStub) GetStream(_ context.Context, _ *entity.Account, streamName string) (*jetstream.StreamInfo, error) {
	return &jetstream.StreamInfo{
		Config:  jetstream.StreamConfig{Name: streamName},
		State:   jetstream.StreamState{Msgs: 100, Consumers: 5},
		Created: time.Now(),
	}, nil
}

func (s *streamerStub) FetchStreamMessages(_ context.Context, _ *entity.Account, _ string, startSeq uint64, batchSize uint32) (*commandv1.StreamMessageBatch, error) {
	var msgs []*commandv1.StreamMessage
	for i := 0; i < int(batchSize); i++ {
		msgs = append(msgs, &commandv1.StreamMessage{
			StreamSequence: uint64(i) + startSeq,
			Timestamp:      time.Now().Unix(),
		})
	}
	return &commandv1.StreamMessageBatch{
		Messages: msgs,
	}, nil
}

func (s *streamerStub) GetStreamMessageContent(_ context.Context, _ *entity.Account, _ string, seq uint64) (*commandv1.StreamMessageContent, error) {
	return &commandv1.StreamMessageContent{
		StreamSequence: seq,
		Timestamp:      time.Now().Unix(),
		Data:           []byte(testutil.RandName()),
	}, nil
}

func (s *streamerStub) ConsumeStream(_ *entity.Account, _ string, _ uint64, handler func(msg *commandv1.ReplyMessage)) (StreamConsumer, error) {
	return newStreamerConsumerStub(s.msgs, handler), nil
}

type streamerConsumerStub struct {
	id             StreamConsumerID
	handler        func(msg *commandv1.ReplyMessage)
	heartbeatCount atomic.Int32
	stop           chan struct{}
}

func newStreamerConsumerStub(msgs <-chan *commandv1.ReplyMessage, handler func(msg *commandv1.ReplyMessage)) *streamerConsumerStub {
	s := &streamerConsumerStub{
		id:             NewStreamConsumerID(),
		handler:        handler,
		heartbeatCount: atomic.Int32{},
		stop:           make(chan struct{}),
	}
	if msgs != nil {
		go func() {
			for {
				select {
				case msg := <-msgs:
					handler(msg)
				case <-s.stop:
					return
				}
			}
		}()
	}
	return s
}

func (s *streamerConsumerStub) ID() string {
	return s.id.String()
}

func (s *streamerConsumerStub) SendHeartbeat(_ context.Context) error {
	s.heartbeatCount.Add(1)
	return nil
}

func (s *streamerConsumerStub) Stop(_ context.Context) error {
	close(s.stop)
	return nil
}

func getStubConsumerHeartbeatCount(t *testing.T, stubConsumer StreamConsumer) int32 {
	s, ok := stubConsumer.(*streamerConsumerStub)
	require.True(t, ok, "underlying stream consumer type is not a streamerStub")
	return s.heartbeatCount.Load()
}
