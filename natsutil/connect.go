package natsutil

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
)

type connectOptions struct {
	tls *tls.Config
}

type ConnectOption func(opts *connectOptions)

func WithTLS(tls *tls.Config) ConnectOption {
	return func(opts *connectOptions) {
		opts.tls = tls
	}
}

func Connect(url string, jwt string, seed string, opts ...ConnectOption) (*nats.Conn, error) {
	const (
		natsConnectTimeout  = 20 * time.Second
		natsConnectAttempts = 10
	)

	var connOpts connectOptions
	for _, opt := range opts {
		opt(&connOpts)
	}

	connectedCh := make(chan struct{})

	natsOpts := []nats.Option{
		nats.UserJWTAndSeed(jwt, seed),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(natsConnectAttempts),
		nats.CustomReconnectDelay(func(_ int) time.Duration {
			return natsConnectTimeout / time.Duration(natsConnectAttempts)
		}),
		nats.ConnectHandler(func(_ *nats.Conn) {
			close(connectedCh)
		}),
	}
	if connOpts.tls != nil {
		natsOpts = append(natsOpts, nats.Secure(connOpts.tls))
	}

	// Connect to the NATS server
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
