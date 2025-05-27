package command

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/natsutil"
	"github.com/coro-sh/coro/syncutil"
)

const (
	maxConsumerIdleHeartbeat  = 30 * time.Second
	consumerHeartbeatInterval = maxConsumerIdleHeartbeat / 2
)

var errConsumerNotFound = errtag.NewTagged[errtag.NotFound]("consumer not found")

type consumerEntry struct {
	heartbeats chan struct{}
	stop       chan struct{}
}

type ConsumerPool struct {
	natsURL          string
	consumers        *syncutil.Map[string, *consumerEntry]
	maxIdleHeartbeat time.Duration
}

func NewConsumerPool(natsURL string) *ConsumerPool {
	return &ConsumerPool{
		natsURL:          natsURL,
		consumers:        syncutil.NewMap[string, *consumerEntry](),
		maxIdleHeartbeat: maxConsumerIdleHeartbeat,
	}
}

func (c *ConsumerPool) StartConsumer(
	ctx context.Context,
	streamName string,
	startSeq uint64,
	consumerID string,
	userJWT string,
	userSeed string,
	handler func(jsMsg jetstream.Msg, cerr error),
) error {
	if c.consumers.Len() >= maxConcurrentConsumers {
		return fmt.Errorf("max concurrent jetstream consumers reached (%d): stop a consumer before trying again", maxConcurrentConsumers)
	}

	consumerNC, err := natsutil.Connect(c.natsURL, userJWT, userSeed)
	if err != nil {
		return fmt.Errorf("connect to nats server using consumer creds: %w", err)
	}

	js, err := jetstream.New(consumerNC)
	if err != nil {
		return fmt.Errorf("create consumer jetstream client: %w", err)
	}

	jsc, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		DeliverPolicy: jetstream.DeliverByStartSequencePolicy,
		AckPolicy:     jetstream.AckNonePolicy,
		OptStartSeq:   startSeq,
		MaxDeliver:    1, // safeguard: shouldn't matter since we are using ack none policy
	})
	if err != nil {
		return fmt.Errorf("create jetstream consumer: %w", err)
	}

	cctx, err := jsc.Consume(func(msg jetstream.Msg) {
		handler(msg, nil)
	}, jetstream.PullMaxMessages(100))
	if err != nil {
		return err
	}

	consumer := &consumerEntry{
		heartbeats: make(chan struct{}, 1),
		stop:       make(chan struct{}),
	}

	c.consumers.Set(consumerID, consumer)

	go func() {
		defer func() {
			c.consumers.Delete(consumerID)
			cctx.Stop()
			consumerNC.Close()
		}()
		for {
			select {
			case <-consumer.heartbeats:
				// consumer is still active - nop
			case <-consumer.stop:
				return
			case <-time.After(c.maxIdleHeartbeat):
				handler(nil, fmt.Errorf("consumer idle heartbeat timeout exceeded (%s)", c.maxIdleHeartbeat))
				return
			}
		}
	}()

	return nil
}

func (c *ConsumerPool) SendConsumerHeartbeat(consumerID string) error {
	consumer, ok := c.consumers.Get(consumerID)
	if !ok {
		return errConsumerNotFound
	}
	select {
	case consumer.heartbeats <- struct{}{}:
	default:
	}
	return nil
}

func (c *ConsumerPool) StopConsumer(consumerID string) error {
	consumer, ok := c.consumers.Get(consumerID)
	if !ok {
		return errConsumerNotFound
	}
	close(consumer.stop)
	return nil
}

func (c *ConsumerPool) StopAll() error {
	for _, consumer := range c.consumers.Keys() {
		if err := c.StopConsumer(consumer); err != nil {
			return err
		}
	}
	return nil
}
