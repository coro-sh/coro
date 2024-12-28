package broker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/coro-sh/coro/log"
)

// SubscriberHandler processes received messages and returns reply data.
type SubscriberHandler func(msg Message[[]byte]) ([]byte, error)

// SubscriberErrorHandler handles any errors that occur during the subscription.
type SubscriberErrorHandler func(err error, metaKeyVals ...any)

type subscriberOptions struct {
	tls        *TLSConfig
	logger     log.Logger
	errHandler SubscriberErrorHandler
}

// TLSConfig holds TLS configuration for secure WebSocket connections.
type TLSConfig struct {
	CertFile           string // Path to the client certificate file.
	KeyFile            string // Path to the client key file.
	CACertFile         string // Path to the CA certificate file.
	InsecureSkipVerify bool   // Allows skipping TLS certificate verification.
}

// SubscriberOption configures a Subscriber.
type SubscriberOption func(opts *subscriberOptions)

// WithSubscriberLogger configures the Subscriber to use the specified logger.
func WithSubscriberLogger(logger log.Logger) SubscriberOption {
	return func(s *subscriberOptions) {
		s.logger = logger
	}
}

// WithSubscriberTLS configures the Subscriber with TLS.
func WithSubscriberTLS(tls TLSConfig) SubscriberOption {
	return func(s *subscriberOptions) {
		s.tls = &tls
	}
}

// WithSubscriberErrorHandler sets the error handler for the Subscriber.
func WithSubscriberErrorHandler(errHandler SubscriberErrorHandler) SubscriberOption {
	return func(s *subscriberOptions) {
		s.errHandler = errHandler
	}
}

// Subscriber subscribes to Operator messages via a WebSocket connection to the
// broker.
type Subscriber struct {
	ws           *websocket.Conn
	errHandler   SubscriberErrorHandler
	logger       log.Logger
	stopped      chan struct{}
	sysUserCreds SysUserCreds
}

// DialSubscriber establishes a WebSocket connection to the Broker and
// initializes a Subscriber to receive messages.
func DialSubscriber(ctx context.Context,
	brokerWebSocketURL string,
	token string,
	opts ...SubscriberOption,
) (*Subscriber, error) {
	options := subscriberOptions{
		logger: log.NewLogger(),
	}
	for _, opt := range opts {
		opt(&options)
	}

	if options.errHandler == nil {
		options.errHandler = func(err error, metaKeyVals ...any) {
			args := append([]any{"error", err}, metaKeyVals...)
			options.logger.Error("broker subscriber error", args...)
		}
	}

	header := make(http.Header)
	header.Set(apiKeyHeader, token)

	httpClient := http.DefaultClient
	if options.tls != nil {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: options.tls.InsecureSkipVerify,
		}

		if options.tls.CertFile != "" && options.tls.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(options.tls.CertFile, options.tls.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("load client certificate/key: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		if options.tls.CACertFile != "" {
			var err error
			tlsConfig.RootCAs, err = loadCACert(options.tls.CACertFile)
			if err != nil {
				return nil, err
			}
		}

		httpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	ws, _, err := websocket.Dial(ctx, brokerWebSocketURL, &websocket.DialOptions{
		HTTPClient:   httpClient,
		HTTPHeader:   header,
		Subprotocols: []string{webSocketSubprotocol},
	})
	if err != nil {
		return nil, err
	}

	// First message must always be system user auth credentials

	var sysUserCreds SysUserCreds
	if err = wsjson.Read(ctx, ws, &sysUserCreds); err != nil {
		return nil, err
	}

	return &Subscriber{
		ws:           ws,
		logger:       options.logger,
		stopped:      make(chan struct{}),
		sysUserCreds: sysUserCreds,
		errHandler:   options.errHandler,
	}, nil
}

// Subscribe starts listening for incoming messages and processes them using
// the provided handler.
func (s *Subscriber) Subscribe(ctx context.Context, handler SubscriberHandler) {
	go func() {
		defer func() { close(s.stopped) }()

		for {
			var msg Message[[]byte]
			if err := wsjson.Read(ctx, s.ws, &msg); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) || websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					return
				}
				s.errHandler(fmt.Errorf("read message: %w", err))
				continue
			}

			errMetaKeyVals := []any{
				log.KeyBrokerMessageID, msg.ID,
				log.KeyBrokerMessageSubject, msg.Subject,
			}

			replyData, err := handler(msg)
			if err != nil {
				s.errHandler(fmt.Errorf("handle message: %w", err), errMetaKeyVals...)
				continue
			}

			replyMsg := Message[json.RawMessage]{
				ID:      msg.ID,
				Inbox:   msg.Inbox,
				Subject: msg.Subject,
				Data:    replyData,
			}

			if err = wsjson.Write(ctx, s.ws, replyMsg); err != nil {
				s.errHandler(fmt.Errorf("write message reply: %w", err), errMetaKeyVals...)
			}
		}
	}()
}

// Unsubscribe closes the WebSocket connection and stops the Subscriber.
func (s *Subscriber) Unsubscribe() error {
	if err := s.ws.Close(websocket.StatusNormalClosure, "subscriber stopped"); err != nil {
		return err
	}
	<-s.stopped
	return nil
}

// SysUserCreds returns the system user credentials associated with the Subscriber.
func (s *Subscriber) SysUserCreds() SysUserCreds {
	return s.sysUserCreds
}
