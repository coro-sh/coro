package natsutil

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/nats-io/nats.go"
)

type connectOptions struct {
	tls                     *tls.Config
	connectHandler          func(*nats.Conn)
	disconnectHandler       func(*nats.Conn, error)
	reconnectHandler        func(*nats.Conn)
	closedHandler           func(*nats.Conn)
	errorHandler            func(*nats.Conn, *nats.Subscription, error)
	maxReconnects           int
	reconnectBackoffInitial time.Duration
	reconnectBackoffMax     time.Duration
	reconnectBackoffJitter  float64
}

type ConnectOption func(opts *connectOptions)

func WithTLS(tls *tls.Config) ConnectOption {
	return func(opts *connectOptions) {
		opts.tls = tls
	}
}

// WithConnectHandler sets a callback that is invoked when the connection is established.
func WithConnectHandler(handler func(*nats.Conn)) ConnectOption {
	return func(opts *connectOptions) {
		opts.connectHandler = handler
	}
}

// WithDisconnectHandler sets a callback that is invoked when the connection is lost.
func WithDisconnectHandler(handler func(*nats.Conn, error)) ConnectOption {
	return func(opts *connectOptions) {
		opts.disconnectHandler = handler
	}
}

// WithReconnectHandler sets a callback that is invoked when the connection is re-established.
func WithReconnectHandler(handler func(*nats.Conn)) ConnectOption {
	return func(opts *connectOptions) {
		opts.reconnectHandler = handler
	}
}

// WithClosedHandler sets a callback that is invoked when the connection is permanently closed.
func WithClosedHandler(handler func(*nats.Conn)) ConnectOption {
	return func(opts *connectOptions) {
		opts.closedHandler = handler
	}
}

// WithErrorHandler sets a callback that is invoked when an async error occurs.
func WithErrorHandler(handler func(*nats.Conn, *nats.Subscription, error)) ConnectOption {
	return func(opts *connectOptions) {
		opts.errorHandler = handler
	}
}

// WithMaxReconnects sets the maximum number of reconnect attempts (-1 for infinite).
func WithMaxReconnects(max int) ConnectOption {
	return func(opts *connectOptions) {
		opts.maxReconnects = max
	}
}

// WithReconnectBackoff sets the exponential backoff parameters for reconnection.
func WithReconnectBackoff(initial, max time.Duration, jitter float64) ConnectOption {
	return func(opts *connectOptions) {
		opts.reconnectBackoffInitial = initial
		opts.reconnectBackoffMax = max
		opts.reconnectBackoffJitter = jitter
	}
}

func Connect(url string, jwt string, seed string, opts ...ConnectOption) (*nats.Conn, error) {
	const (
		natsConnectTimeout                = 20 * time.Second
		defaultMaxReconnects              = -1 // Infinite reconnects
		defaultReconnectBackoffInitial    = 1 * time.Second
		defaultReconnectBackoffMax        = 10 * time.Minute
		defaultReconnectBackoffJitter     = 0.2
		defaultReconnectBackoffMultiplier = 2.0
	)

	connOpts := connectOptions{
		maxReconnects:           defaultMaxReconnects,
		reconnectBackoffInitial: defaultReconnectBackoffInitial,
		reconnectBackoffMax:     defaultReconnectBackoffMax,
		reconnectBackoffJitter:  defaultReconnectBackoffJitter,
	}
	for _, opt := range opts {
		opt(&connOpts)
	}

	connectedCh := make(chan struct{})
	var connectedOnce bool

	attempts := 0
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = connOpts.reconnectBackoffInitial
	b.MaxInterval = connOpts.reconnectBackoffMax
	b.Multiplier = defaultReconnectBackoffMultiplier
	b.RandomizationFactor = connOpts.reconnectBackoffJitter
	b.Reset()

	reconnectDelay := func(reconnects int) time.Duration {
		attempts++
		return b.NextBackOff()
	}

	natsOpts := []nats.Option{
		nats.UserJWTAndSeed(jwt, seed),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(connOpts.maxReconnects),
		nats.CustomReconnectDelay(reconnectDelay),
		nats.ConnectHandler(func(nc *nats.Conn) {
			if !connectedOnce {
				close(connectedCh)
				connectedOnce = true
			}
			// Reset backoff attempts on successful connection
			attempts = 0
			if connOpts.connectHandler != nil {
				connOpts.connectHandler(nc)
			}
		}),
	}

	if connOpts.disconnectHandler != nil {
		natsOpts = append(natsOpts, nats.DisconnectErrHandler(connOpts.disconnectHandler))
	}

	if connOpts.reconnectHandler != nil {
		natsOpts = append(natsOpts, nats.ReconnectHandler(connOpts.reconnectHandler))
	}

	if connOpts.closedHandler != nil {
		natsOpts = append(natsOpts, nats.ClosedHandler(connOpts.closedHandler))
	}

	if connOpts.errorHandler != nil {
		natsOpts = append(natsOpts, nats.ErrorHandler(connOpts.errorHandler))
	}

	if connOpts.tls != nil {
		natsOpts = append(natsOpts, nats.Secure(connOpts.tls))
	}

	nc, err := nats.Connect(url, natsOpts...)
	if err != nil {
		return nil, err
	}

	select {
	case <-connectedCh:
	case <-time.After(natsConnectTimeout):
		nc.Close()
		return nil, errors.New("nats connect timeout")
	}

	return nc, nil
}
