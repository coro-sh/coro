package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/coder/websocket"
	"github.com/joshjon/kit/log"
)

// ReconnectableSubscriber wraps a Subscriber with automatic reconnection logic.
// It monitors the connection and automatically reconnects with exponential backoff
// when the connection is lost after the initial successful connection.
type ReconnectableSubscriber struct {
	brokerURL string
	token     string
	baseOpts  []SubscriberOption
	logger    log.Logger
	backoff   backoff.BackOff

	mu         sync.RWMutex
	subscriber *Subscriber
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewReconnectableSubscriber creates a new ReconnectableSubscriber with the given
// broker WebSocket URL, token, and options.
func NewReconnectableSubscriber(ctx context.Context,
	brokerURL string,
	token string,
	opts ...SubscriberOption,
) (*ReconnectableSubscriber, error) {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 1 * time.Second
	b.MaxInterval = 10 * time.Minute
	b.Multiplier = 2.0
	b.RandomizationFactor = 0.2
	b.Reset()

	rs := &ReconnectableSubscriber{
		brokerURL: brokerURL,
		token:     token,
		baseOpts:  opts,
		logger:    log.NewLogger(),
		backoff:   b,
	}

	for _, opt := range opts {
		tmpOpts := &commandSubscriberOptions{}
		opt(tmpOpts)
		if tmpOpts.logger != nil {
			rs.logger = tmpOpts.logger
		}
	}

	rs.logger.Info("establishing initial broker websocket connection", "url", brokerURL)
	sub, err := NewCommandSubscriber(ctx, brokerURL, token, opts...)
	if err != nil {
		return nil, fmt.Errorf("initial broker connection failed: %w", err)
	}

	rs.subscriber = sub
	rs.logger.Info("broker websocket connection established successfully")

	return rs, nil
}

// Subscribe starts listening for messages with automatic reconnection.
// When the connection is lost, it will automatically attempt to reconnect
// with exponential backoff.
func (rs *ReconnectableSubscriber) Subscribe(ctx context.Context, handler SubscriberHandler) {
	rs.ctx, rs.cancel = context.WithCancel(ctx)
	go rs.subscribeLoop(handler)
}

func (rs *ReconnectableSubscriber) subscribeLoop(handler SubscriberHandler) {
	for {
		select {
		case <-rs.ctx.Done():
			rs.logger.Info("broker subscriber stopped due to context cancellation")
			return
		default:
		}

		rs.mu.RLock()
		sub := rs.subscriber
		rs.mu.RUnlock()

		if sub == nil {
			rs.logger.Warn("no active broker connection, attempting to reconnect")
			if err := rs.reconnect(); err != nil {
				rs.logger.Error("failed to reconnect to broker", "error", err)
				continue
			}
			rs.mu.RLock()
			sub = rs.subscriber
			rs.mu.RUnlock()
		}

		subError := make(chan error, 1)
		errorHandler := func(err error, metaKeyVals ...any) {
			args := append([]any{"error", err}, metaKeyVals...)
			rs.logger.Error("broker subscriber error", args...)

			// Check if this is a connection error (not graceful shutdown)
			if !errors.Is(err, context.Canceled) &&
				!errors.Is(err, net.ErrClosed) &&
				!errors.Is(err, io.EOF) &&
				websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				select {
				case subError <- err:
				default:
				}
			}
		}

		opts := append(rs.baseOpts, WithSubscriberErrorHandler(errorHandler))
		subWithErrorHandler := rs.createSubscriberWithOptions(sub, opts)

		subWithErrorHandler.Subscribe(rs.ctx, handler)

		// Wait for error, subscription stop, or cancellation
		select {
		case err := <-subError:
			rs.logger.Warn("broker connection lost, will attempt reconnection", "error", err)
			rs.mu.Lock()
			if rs.subscriber != nil {
				rs.subscriber.Unsubscribe() //nolint:errcheck
				rs.subscriber = nil
			}
			rs.mu.Unlock()
			// Wait for subscription to fully stop before reconnecting
			<-subWithErrorHandler.stopped
		case <-subWithErrorHandler.stopped:
			if rs.ctx.Err() != nil {
				rs.logger.Info("broker subscription stopped due to context cancellation")
				return
			}
			// Connection may have been lost without triggering error handler
			rs.logger.Warn("broker subscription stopped unexpectedly, will attempt reconnection")
			rs.mu.Lock()
			if rs.subscriber != nil {
				rs.subscriber.Unsubscribe() //nolint:errcheck
				rs.subscriber = nil
			}
			rs.mu.Unlock()
		case <-rs.ctx.Done():
			return
		}
	}
}

func (rs *ReconnectableSubscriber) createSubscriberWithOptions(sub *Subscriber, opts []SubscriberOption) *Subscriber {
	subCopy := &Subscriber{
		ws:           sub.ws,
		logger:       sub.logger,
		stopped:      sub.stopped,
		sysUserCreds: sub.sysUserCreds,
	}

	optsStruct := &commandSubscriberOptions{
		logger: sub.logger,
	}
	for _, opt := range opts {
		opt(optsStruct)
	}

	if optsStruct.errHandler != nil {
		subCopy.errHandler = optsStruct.errHandler
	}
	if optsStruct.logger != nil {
		subCopy.logger = optsStruct.logger
	}

	return subCopy
}

func (rs *ReconnectableSubscriber) reconnect() error {
	attempt := 0
	for {
		backoffDuration := rs.backoff.NextBackOff()
		attempt++

		rs.logger.Info("attempting broker reconnection",
			"attempt", attempt,
			"backoff_duration", backoffDuration.String(),
		)

		// Wait for backoff duration or context cancellation
		select {
		case <-time.After(backoffDuration):
			// Continue with reconnection attempt
		case <-rs.ctx.Done():
			return fmt.Errorf("reconnection canceled: %w", rs.ctx.Err())
		}

		sub, err := NewCommandSubscriber(rs.ctx, rs.brokerURL, rs.token, rs.baseOpts...)
		if err != nil {
			nextBackoff := rs.backoff.NextBackOff()
			rs.logger.Error("broker reconnection attempt failed",
				"error", err,
				"attempt", attempt,
				"next_backoff", nextBackoff.String(),
			)
			continue
		}

		rs.logger.Info("broker websocket reconnection successful", "total_attempts", attempt)
		rs.backoff.Reset()

		rs.mu.Lock()
		rs.subscriber = sub
		rs.mu.Unlock()

		return nil
	}
}

// Unsubscribe stops the subscriber and cancels any reconnection attempts.
func (rs *ReconnectableSubscriber) Unsubscribe() error {
	if rs.cancel != nil {
		rs.cancel()
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.subscriber != nil {
		return rs.subscriber.Unsubscribe()
	}
	return nil
}

// SysUserCreds returns the system user credentials from the current subscriber.
func (rs *ReconnectableSubscriber) SysUserCreds() UserCreds {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if rs.subscriber != nil {
		return rs.subscriber.SysUserCreds()
	}
	return UserCreds{}
}

// IsConnected returns true if the subscriber is currently connected.
func (rs *ReconnectableSubscriber) IsConnected() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.subscriber != nil
}
