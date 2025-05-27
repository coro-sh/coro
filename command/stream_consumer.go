package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/log"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
)

type streamConsumer struct {
	nc           *nats.Conn
	operatorID   entity.OperatorID
	consumerUser *entity.User
	streamName   string

	started    bool
	consumerID string
	replySub   *nats.Subscription
	logger     log.Logger
}

func newStreamConsumer(nc *nats.Conn, operatorID entity.OperatorID, consumerUser *entity.User, streamName string) *streamConsumer {
	return &streamConsumer{
		nc:           nc,
		operatorID:   operatorID,
		consumerUser: consumerUser,
		streamName:   streamName,
		consumerID:   NewStreamConsumerID().String(),
		logger:       log.NewLogger(),
	}
}

func (c *streamConsumer) ID() string {
	return c.consumerID
}

func (c *streamConsumer) Start(startSeq uint64, handler func(msg *commandv1.ReplyMessage)) error {
	if c.started {
		return errors.New("consumer already started")
	}
	if startSeq == 0 {
		startSeq = 1
	}

	msg := &commandv1.PublishMessage{
		Id:                NewMessageID().String(),
		CommandReplyInbox: nats.NewInbox(),
		Command: &commandv1.PublishMessage_StartStreamConsumer{
			StartStreamConsumer: &commandv1.PublishMessage_CommandStartStreamConsumer{
				UserCreds: &commandv1.Credentials{
					Jwt:  c.consumerUser.JWT(),
					Seed: string(c.consumerUser.NKey().Seed),
				},
				ConsumerId:    c.consumerID,
				StreamName:    c.streamName,
				StartSequence: startSeq,
			},
		},
	}
	msgb, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	c.replySub, err = c.nc.Subscribe(msg.CommandReplyInbox, func(msg *nats.Msg) {
		replyMsg := &commandv1.ReplyMessage{}
		if err := proto.Unmarshal(msg.Data, replyMsg); err != nil {
			c.logger.Error("failed to unmarshal reply message", "error", err)
			return
		}
		handler(replyMsg)
	})
	if err != nil {
		return fmt.Errorf("subscribe consumer reply inbox: %w", err)
	}

	if err = c.nc.PublishRequest(getOperatorSubject(c.operatorID), msg.CommandReplyInbox, msgb); err != nil {
		err = fmt.Errorf("publish message: %w", err)
		return errors.Join(err, c.replySub.Unsubscribe())
	}

	c.started = true
	return nil
}

func (c *streamConsumer) SendHeartbeat(ctx context.Context) error {
	if !c.started {
		return errors.New("consumer not started")
	}
	_, err := command(ctx, c.nc, c.operatorID, &commandv1.PublishMessage{
		Id:                NewMessageID().String(),
		CommandReplyInbox: NewMessageID().String(),
		Command: &commandv1.PublishMessage_SendStreamConsumerHeartbeat{
			SendStreamConsumerHeartbeat: &commandv1.PublishMessage_CommandSendStreamConsumerHeartbeat{
				ConsumerId: c.consumerID,
			},
		},
	})
	return err
}

func (c *streamConsumer) Stop(ctx context.Context) error {
	if !c.started {
		return nil
	}
	if c.replySub != nil {
		if err := c.replySub.Unsubscribe(); err != nil {
			return fmt.Errorf("unsubscribe consumer reply inbox: %w", err)
		}
		c.replySub = nil
	}

	msgb, err := command(ctx, c.nc, c.operatorID, &commandv1.PublishMessage{
		Id:                NewMessageID().String(),
		CommandReplyInbox: NewMessageID().String(),
		Command: &commandv1.PublishMessage_StopStreamConsumer{
			StopStreamConsumer: &commandv1.PublishMessage_CommandStopStreamConsumer{
				ConsumerId: c.consumerID,
			},
		},
	})
	if err != nil {
		return err
	}
	replyMsg := &commandv1.ReplyMessage{}
	if err := proto.Unmarshal(msgb, replyMsg); err != nil {
		return fmt.Errorf("unmarshal reply message: %w", err)
	}

	if replyMsg.Error != nil && *replyMsg.Error != errConsumerNotFound.Error() {
		return fmt.Errorf("reply: %s", *replyMsg.Error)
	}

	c.started = false
	return nil
}
