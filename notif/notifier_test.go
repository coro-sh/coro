package notif

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/internal/testutil"
)

func TestNotifier_NotifyAccountClaimsUpdate(t *testing.T) {
	ctx := context.Background()
	mockStore := entity.NewStore(new(testutil.FakeTxer), entity.NewFakeEntityRepository(t))

	op, err := entity.NewOperator(testutil.RandName(), entity.NewID[entity.NamespaceID]())
	require.NoError(t, err)
	acc, err := entity.NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	accData, err := acc.Data()
	require.NoError(t, err)

	wantReply, err := json.Marshal(AccountUpdateReplyMessage{
		Data: struct {
			Code    int    `json:"code"`
			Account string `json:"account"`
			Message string `json:"message"`
		}{
			Code:    200,
			Account: accData.PublicKey,
			Message: "jwt updated",
		},
	})
	require.NoError(t, err)

	notifier, err := NewNotifier(mockStore, &stubPublisher{reply: wantReply})
	require.NoError(t, err)

	err = notifier.NotifyAccountClaimsUpdate(ctx, acc)
	require.NoError(t, err)
}

func TestNotifier_Ping(t *testing.T) {
	ctx := context.Background()
	mockStore := new(entity.Store)

	notifier, err := NewNotifier(mockStore, new(stubPublisher))
	require.NoError(t, err)

	op, err := entity.NewOperator(testutil.RandName(), entity.NewID[entity.NamespaceID]())
	require.NoError(t, err)

	status, err := notifier.Ping(ctx, op)
	require.NoError(t, err)
	assert.True(t, status.Connected)
	assert.Positive(t, *status.ConnectTime)
}

type stubPublisher struct {
	reply []byte
}

func (m *stubPublisher) Publish(_ context.Context, _ entity.OperatorID, _ string, _ []byte) ([]byte, error) {
	return m.reply, nil
}

func (m *stubPublisher) Ping(_ context.Context, _ entity.OperatorID) (entity.OperatorNATSStatus, error) {
	connectTime := time.Now().Unix()
	return entity.OperatorNATSStatus{
		Connected:   true,
		ConnectTime: &connectTime,
	}, nil
}
