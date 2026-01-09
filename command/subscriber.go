package command

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/joshjon/kit/log"
	"google.golang.org/protobuf/proto"

	"github.com/coro-sh/coro/logkey"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
)

type SubscriptionReplier func(ctx context.Context, msg *commandv1.ReplyMessage) error

// SubscriberHandler processes received messages and returns reply(s).
type SubscriberHandler func(msg *commandv1.PublishMessage, replier SubscriptionReplier) error

// SubscriberErrorHandler handles any errors that occur during the subscription.
type SubscriberErrorHandler func(err error, metaKeyVals ...any)

type commandSubscriberOptions struct {
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
type SubscriberOption func(opts *commandSubscriberOptions)

// WithCommandSubscriberLogger configures the Subscriber to use the
// specified logger.
func WithCommandSubscriberLogger(logger log.Logger) SubscriberOption {
	return func(s *commandSubscriberOptions) {
		s.logger = logger
	}
}

// WithCommandSubscriberTLS configures the Subscriber with TLS.
func WithCommandSubscriberTLS(tls TLSConfig) SubscriberOption {
	return func(s *commandSubscriberOptions) {
		s.tls = &tls
	}
}

// WithSubscriberErrorHandler sets the error handler for the Subscriber.
func WithSubscriberErrorHandler(errHandler SubscriberErrorHandler) SubscriberOption {
	return func(s *commandSubscriberOptions) {
		s.errHandler = errHandler
	}
}

// Subscriber subscribes to Operator messages via a WebSocket connection
// to the broker.
type Subscriber struct {
	ws           *websocket.Conn
	errHandler   SubscriberErrorHandler
	logger       log.Logger
	stopped      chan struct{}
	sysUserCreds UserCreds
}

// NewCommandSubscriber establishes a WebSocket connection to the Broker and
// initializes a Subscriber to receive messages.
func NewCommandSubscriber(ctx context.Context,
	brokerWebSocketURL string,
	token string,
	opts ...SubscriberOption,
) (*Subscriber, error) {
	options := commandSubscriberOptions{
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
		Subprotocols: []string{brokerWebSocketSubprotocol},
	})
	if err != nil {
		return nil, err
	}

	// First message must always be system user auth credentials

	var sysUserCreds UserCreds
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
			_, pubMsgb, err := s.ws.Read(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) || websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					return
				}
				s.errHandler(fmt.Errorf("subscriber stopped: %w", err))
				return
			}

			pubMsg := &commandv1.PublishMessage{}
			if err = proto.Unmarshal(pubMsgb, pubMsg); err != nil {
				s.logger.Error("failed to unmarshal received message", "error", err)
				continue
			}

			var errMetaKeyVals []any

			err = func() error {
				opType, err := getOperationType(pubMsg)
				if err != nil {
					return err
				}

				errMetaKeyVals = append(errMetaKeyVals,
					logkey.BrokerMessageID, pubMsg.Id,
					logkey.BrokerMessageOperation, opType,
				)

				err = handler(pubMsg, func(ctx context.Context, replyMsg *commandv1.ReplyMessage) error {
					reply, err := proto.Marshal(replyMsg)
					if err != nil {
						return fmt.Errorf("marshal reply message: %w", err)
					}
					return s.ws.Write(ctx, websocket.MessageText, reply)
				})
				if err != nil {
					return err
				}
				return nil
			}()
			if err != nil {
				s.errHandler(err, errMetaKeyVals...)
				errStr := err.Error()
				reply, err := proto.Marshal(&commandv1.ReplyMessage{
					Id:    pubMsg.Id,
					Inbox: pubMsg.CommandReplyInbox,
					Error: &errStr,
				})
				if err != nil {
					s.errHandler(fmt.Errorf("marshal error reply message: %w", err), errMetaKeyVals...)
					continue
				}
				if err = s.ws.Write(ctx, websocket.MessageText, reply); err != nil {
					s.errHandler(fmt.Errorf("send error reply message: %w", err), errMetaKeyVals...)
				}
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
func (s *Subscriber) SysUserCreds() UserCreds {
	return s.sysUserCreds
}
