package command

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/log"
	"github.com/coro-sh/coro/natsutil"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
)

const (
	natsRequestTimeout     = 10 * time.Second
	maxConcurrentConsumers = 10
)

type proxyOptions struct {
	logger          log.Logger
	brokerTLSConfig *TLSConfig
	natsTLSConfig   *tls.Config
}

// ProxyOption configure a Proxy.
type ProxyOption func(opts *proxyOptions)

// WithProxyLogger configures the Proxy to use the specified logger.
func WithProxyLogger(logger log.Logger) ProxyOption {
	return func(opts *proxyOptions) {
		opts.logger = logger
	}
}

// WithProxyBrokerTLS sets the TLS configuration for the Broker WebSocket
// connection.
func WithProxyBrokerTLS(tlsConfig TLSConfig) ProxyOption {
	return func(opts *proxyOptions) {
		opts.brokerTLSConfig = &tlsConfig
	}
}

// WithProxyNatsTLS sets the TLS configuration for the NATS connection.
func WithProxyNatsTLS(tlsConfig *tls.Config) ProxyOption {
	return func(opts *proxyOptions) {
		opts.natsTLSConfig = tlsConfig
	}
}

// Proxy establishes a connection between a Broker WebSocket server and a NATS
// server, facilitating message forwarding between the two.
type Proxy struct {
	cmdSub       *CommandSubscriber
	natsURL      string
	nc           *nats.Conn
	consumerPool *ConsumerPool
	logger       log.Logger
}

// NewProxy creates a Proxy by establishing a proxy connection by connecting to both
// a NATS server and a Broker WebSocket server. It authenticates the WebSocket
// connection using the provided proxy token.
func NewProxy(ctx context.Context, natsURL string, brokerWebSocketURL string, token string, opts ...ProxyOption) (*Proxy, error) {
	options := proxyOptions{
		logger: log.NewLogger(),
	}
	for _, opt := range opts {
		opt(&options)
	}

	// Broker subscriber
	brokerOpts := []SubscriberOption{
		WithSubscriberLogger(options.logger),
	}
	if options.brokerTLSConfig != nil {
		brokerOpts = append(brokerOpts, WithSubscriberTLS(*options.brokerTLSConfig))
	}
	cmdSub, err := NewCommandSubscriber(ctx, brokerWebSocketURL, token, brokerOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial broker subscriber: %w", err)
	}
	sysUserCreds := cmdSub.SysUserCreds()

	nc, err := natsutil.Connect(natsURL, sysUserCreds.JWT, sysUserCreds.Seed, natsutil.WithTLS(options.natsTLSConfig))
	if err != nil {
		return nil, fmt.Errorf("connect to nats server: %w", err)
	}

	return &Proxy{
		cmdSub:       cmdSub,
		natsURL:      natsURL,
		nc:           nc,
		consumerPool: NewConsumerPool(natsURL),
		logger:       options.logger,
	}, nil
}

// Start begins processing Broker WebSocket messages and forwarding them to the
// connected NATS server.
func (p *Proxy) Start(ctx context.Context) {
	p.cmdSub.Subscribe(ctx, func(msg *commandv1.PublishMessage, replier SubscriptionReplier) error {
		return p.handleMessage(ctx, msg, replier)
	})
}

// Stop terminates any message forwarding started on the proxy.
func (p *Proxy) Stop() error {
	err := p.cmdSub.Unsubscribe()
	p.nc.Close()
	return err
}

func (p *Proxy) handleMessage(ctx context.Context, msg *commandv1.PublishMessage, replier SubscriptionReplier) (err error) {
	replyFailedErr := errors.New("reply failed")
	reply := func(data []byte) error {
		if rerr := replier(ctx, &commandv1.ReplyMessage{
			Id:    msg.Id,
			Inbox: msg.CommandReplyInbox,
			Data:  data,
		}); rerr != nil {
			return fmt.Errorf("%w: %v", replyFailedErr, rerr)
		}
		return nil
	}

	// Only reply with handler error if it wasn't caused by another reply
	defer func() {
		if err != nil && !errors.Is(err, replyFailedErr) {
			errStr := err.Error()
			if rerr := replier(ctx, &commandv1.ReplyMessage{
				Id:    msg.Id,
				Inbox: msg.CommandReplyInbox,
				Error: &errStr,
			}); rerr != nil {
				p.logger.Error("failed to reply to command with error message", "error", rerr, "handler_error", err.Error())
			}
		}
	}()

	if op := msg.GetRequest(); op != nil {
		if msg.CommandReplyInbox == "" {
			if err := p.nc.Publish(op.Subject, op.Data); err != nil {
				return fmt.Errorf("publish mesage to nats server: %w", err)
			}
			return nil
		}
		natsReply, err := p.nc.Request(op.Subject, op.Data, natsRequestTimeout)
		if err != nil {
			return fmt.Errorf("request nats server: %w", err)
		}
		return reply(natsReply.Data)
	}

	if op := msg.GetListStream(); op != nil {
		if op.UserCreds == nil {
			return errors.New("missing user creds for list stream operation")
		}
		nc, err := natsutil.Connect(p.natsURL, op.UserCreds.Jwt, op.UserCreds.Seed)
		if err != nil {
			return fmt.Errorf("connect to nats server using operation creds: %w", err)
		}
		js, err := jetstream.New(nc)
		if err != nil {
			return fmt.Errorf("create jetstream client: %w", err)
		}

		lctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var infos []*jetstream.StreamInfo
		lis := js.ListStreams(lctx)
		for info := range lis.Info() {
			infos = append(infos, info)
		}
		if lis.Err() != nil {
			return fmt.Errorf("list streams: %w", lis.Err())
		}

		replyData, err := json.Marshal(infos)
		if err != nil {
			return fmt.Errorf("marshal list streams reply data: %w", err)
		}

		return reply(replyData)
	}

	if op := msg.GetStartConsumer(); op != nil {
		if op.UserCreds == nil {
			return errors.New("missing user creds for start consumer operation")
		}

		userCreds := op.UserCreds
		consumerID := op.ConsumerId
		err = p.consumerPool.StartConsumer(ctx, op.StreamName, consumerID, userCreds.Jwt, userCreds.Seed, func(jsMsg jetstream.Msg, cerr error) {
			replyMsg := &commandv1.ReplyMessage{
				Id:    msg.Id,
				Inbox: msg.CommandReplyInbox,
			}

			if jsMsg != nil {
				replyMsg.Data = jsMsg.Data()
				if md, err := jsMsg.Metadata(); err == nil {
					replyMsg.Id += "_" + strconv.Itoa(int(md.Sequence.Consumer))
				}
			}
			if cerr != nil {
				errStr := cerr.Error()
				replyMsg.Error = &errStr
			}
			if err := replier(ctx, replyMsg); err != nil {
				p.logger.Error(
					"failed to forward jetstream ephemeral consumer message to broker",
					"error", err,
					"reply_message.id", replyMsg.Id,
				)
			}
		})
		if err != nil {
			return err
		}

		return reply(nil)
	}

	if op := msg.GetSendConsumerHeartbeat(); op != nil {
		if err := p.consumerPool.SendConsumerHeartbeat(op.ConsumerId); err != nil {
			return err
		}
		return reply(nil)
	}

	if op := msg.GetStopConsumer(); op != nil {
		if err := p.consumerPool.StopConsumer(op.ConsumerId); err != nil && !errtag.HasTag[errtag.NotFound](err) {
			return err
		}
		return reply(nil)
	}

	return errors.New("no operation found in message")
}
