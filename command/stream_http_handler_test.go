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

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/internal/testutil"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
	"github.com/coro-sh/coro/server"
)

func TestStreamHTTPHandler_ListStreams(t *testing.T) {
	ctx := t.Context()

	now := time.Now()

	want := []StreamResponse{
		{
			Name:          "stream_1",
			Subjects:      []string{"subject_foo", "subject_bar"},
			MessageCount:  100,
			ConsumerCount: 5,
			CreateTime:    now.UnixMilli(),
		},
		{
			Name:          "stream_2",
			Subjects:      []string{"subject_lorem", "subject_ipsum"},
			MessageCount:  1000,
			ConsumerCount: 50,
			CreateTime:    now.UnixMilli(),
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

	op, err := entity.NewOperator(testutil.RandName(), entity.NewID[entity.NamespaceID]())
	require.NoError(t, err)
	acc, err := entity.NewAccount(testutil.RandName(), op)
	err = entityStore.CreateAccount(ctx, acc)
	require.NoError(t, err)

	url := fmt.Sprintf("%s%s%s/namespaces/%s/accounts/%s/streams", srv.Address(), server.APIPath, entity.VersionPath, op.NamespaceID, acc.ID)

	res := testutil.Get[server.ResponseList[StreamResponse]](t, url)
	got := res.Data
	assert.Equal(t, want, got)
	assert.Nil(t, res.NextPageCursor)
}

func newStreamHTTPServer(t *testing.T, streams []*jetstream.StreamInfo, msgs <-chan *commandv1.ReplyMessage) (*server.Server, *entity.Store) {
	entityStore := entity.NewStore(new(testutil.FakeTxer), entity.NewFakeEntityRepository(t))

	srv, err := server.NewServer(testutil.GetFreePort(t), server.WithMiddleware(entity.NamespaceContextMiddleware()))
	require.NoError(t, err)
	srv.Register(NewStreamHTTPHandler(entityStore, newStreamerStub(streams, msgs)))

	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, srv.Stop(t.Context()))
	})

	return srv, entityStore
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

func (s *streamerStub) ConsumeStream(_ *entity.Account, _ string, handler func(msg *commandv1.ReplyMessage)) (StreamConsumer, error) {
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
