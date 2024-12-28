package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/coro-sh/coro/broker"
	"github.com/coro-sh/coro/log"
)

const natsRequestTimeout = 10 * time.Second

type proxyOptions struct {
	logger          log.Logger
	brokerTLSConfig *broker.TLSConfig
	natsTLSConfig   *tls.Config
}

// Option configure a Proxy.
type Option func(opts *proxyOptions)

// WithLogger configures the Proxy to use the specified logger.
func WithLogger(logger log.Logger) Option {
	return func(opts *proxyOptions) {
		opts.logger = logger
	}
}

// WithBrokerTLS sets the TLS configuration for the Broker WebSocket
// connection.
func WithBrokerTLS(tlsConfig broker.TLSConfig) Option {
	return func(opts *proxyOptions) {
		opts.brokerTLSConfig = &tlsConfig
	}
}

// WithNatsTLS sets the TLS configuration for the NATS connection.
func WithNatsTLS(tlsConfig *tls.Config) Option {
	return func(opts *proxyOptions) {
		opts.natsTLSConfig = tlsConfig
	}
}

// Proxy establishes a connection between a Broker WebSocket server and a NATS
// server, facilitating message forwarding between the two.
type Proxy struct {
	brokerSub *broker.Subscriber
	nc        *nats.Conn
	logger    log.Logger
}

// Dial creates a Proxy by establishing a proxy connection by connecting to both
// a NATS server and a Broker WebSocket server. It authenticates the WebSocket
// connection using the provided proxy token.
func Dial(ctx context.Context, natsURL string, brokerWebSocketURL string, token string, opts ...Option) (*Proxy, error) {
	options := proxyOptions{
		logger: log.NewLogger(),
	}
	for _, opt := range opts {
		opt(&options)
	}

	// Broker subscriber
	brokerOpts := []broker.SubscriberOption{
		broker.WithSubscriberLogger(options.logger),
	}
	if options.brokerTLSConfig != nil {
		brokerOpts = append(brokerOpts, broker.WithSubscriberTLS(*options.brokerTLSConfig))
	}
	brokerSub, err := broker.DialSubscriber(ctx, brokerWebSocketURL, token, brokerOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial broker subscriber: %w", err)
	}
	sysUserCreds := brokerSub.SysUserCreds()

	var once sync.Once
	connectedCh := make(chan struct{})

	connectTimeout := 20 * time.Second
	connectAttempts := 10

	// NATS client
	natsOpts := []nats.Option{
		nats.UserJWTAndSeed(sysUserCreds.JWT, sysUserCreds.Seed),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.CustomReconnectDelay(func(_ int) time.Duration {
			return connectTimeout / time.Duration(connectAttempts)
		}),
		nats.ConnectHandler(func(_ *nats.Conn) {
			once.Do(func() { close(connectedCh) })
		}),
	}
	if options.natsTLSConfig != nil {
		natsOpts = append(natsOpts, nats.Secure(options.natsTLSConfig))
	}

	// Connect to the NATS server
	nc, err := nats.Connect(natsURL, natsOpts...)
	if err != nil {
		return nil, err
	}

	select {
	case <-connectedCh:
	case <-time.After(connectTimeout):
		nc.Close()
		return nil, errors.New("nats connect timeout")
	}

	return &Proxy{
		brokerSub: brokerSub,
		nc:        nc,
		logger:    options.logger,
	}, nil
}

// Start begins processing Broker WebSocket messages and forwarding them to the
// connected NATS server.
func (p *Proxy) Start(ctx context.Context) {
	brokerSubHandler := func(msg broker.Message[[]byte]) ([]byte, error) {
		replyMsg, err := p.nc.Request(msg.Subject, msg.Data, natsRequestTimeout)
		if err != nil {
			return nil, fmt.Errorf("forward message to nats server: %w", err)
		}
		return replyMsg.Data, nil
	}

	p.brokerSub.Subscribe(ctx, brokerSubHandler)
}

// Stop terminates any message forwarding started on the proxy.
func (p *Proxy) Stop() error {
	err := p.brokerSub.Unsubscribe()
	p.nc.Close()
	return err
}
