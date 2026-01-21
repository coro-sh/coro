package command

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	"github.com/joshjon/kit/errtag"
	"github.com/joshjon/kit/log"

	"github.com/coro-sh/coro/natsutil"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
)

const (
	natsRequestTimeout     = 10 * time.Second
	maxConcurrentConsumers = 10
	messageHandlerTimeout  = 5 * time.Second
)

type proxyOptions struct {
	logger              log.Logger
	brokerTLSConfig     *TLSConfig
	natsTLSConfig       *tls.Config
	brokerCustomHeaders http.Header
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

// WithProxyBrokerCustomHeaders sets custom HTTP headers for the Broker WebSocket connection.
func WithProxyBrokerCustomHeaders(headers http.Header) ProxyOption {
	return func(opts *proxyOptions) {
		opts.brokerCustomHeaders = headers
	}
}

// Proxy establishes a connection between a Broker WebSocket server and a NATS
// server, facilitating message forwarding between the two.
type Proxy struct {
	cmdSub       *Subscriber
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
		WithCommandSubscriberLogger(options.logger),
	}
	if options.brokerTLSConfig != nil {
		brokerOpts = append(brokerOpts, WithCommandSubscriberTLS(*options.brokerTLSConfig))
	}
	if options.brokerCustomHeaders != nil {
		brokerOpts = append(brokerOpts, WithCommandSubscriberCustomHeaders(options.brokerCustomHeaders))
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
	ctx, cancel := context.WithTimeout(ctx, messageHandlerTimeout)
	defer cancel()

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

	type userCredsGetter interface{ GetUserCreds() *commandv1.Credentials }
	connectUser := func(ucg userCredsGetter) (*nats.Conn, jetstream.JetStream, error) {
		creds := ucg.GetUserCreds()
		if creds == nil {
			return nil, nil, errors.New("user creds cannot be nil")
		}
		nc, err := natsutil.Connect(p.natsURL, creds.Jwt, creds.Seed)
		if err != nil {
			return nil, nil, fmt.Errorf("connect to nats server using operation creds: %w", err)
		}
		js, err := jetstream.New(nc)
		if err != nil {
			return nil, nil, fmt.Errorf("create jetstream client: %w", err)
		}
		return nc, js, nil
	}

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
		nc, js, err := connectUser(op)
		if err != nil {
			return err
		}
		defer nc.Close()

		var infos []*jetstream.StreamInfo
		lis := js.ListStreams(ctx)
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

	if op := msg.GetGetStream(); op != nil {
		nc, js, err := connectUser(op)
		if err != nil {
			return err
		}
		defer nc.Close()

		stream, err := js.Stream(ctx, op.StreamName)
		if err != nil {
			return fmt.Errorf("get stream: %w", err)
		}
		info, err := stream.Info(ctx)
		if err != nil {
			return fmt.Errorf("get stream info: %w", err)
		}

		replyData, err := json.Marshal(info)
		if err != nil {
			return fmt.Errorf("marshal get stream reply data: %w", err)
		}

		return reply(replyData)
	}

	if op := msg.GetFetchStreamMessages(); op != nil {
		if op.UserCreds == nil {
			return errors.New("missing user creds for start consumer operation")
		}
		consumerNC, err := natsutil.Connect(p.natsURL, op.UserCreds.Jwt, op.UserCreds.Seed)
		if err != nil {
			return fmt.Errorf("connect to nats server using consumer creds: %w", err)
		}
		defer consumerNC.Close()

		js, err := jetstream.New(consumerNC)
		if err != nil {
			return fmt.Errorf("create consumer jetstream client: %w", err)
		}

		jsc, err := js.CreateOrUpdateConsumer(ctx, op.StreamName, jetstream.ConsumerConfig{
			DeliverPolicy: jetstream.DeliverByStartSequencePolicy,
			OptStartSeq:   op.StartSequence,
			AckPolicy:     jetstream.AckNonePolicy,
			MaxDeliver:    1, // safeguard: shouldn't matter since we are using ack none policy
		})
		if err != nil {
			return fmt.Errorf("create jetstream consumer: %w", err)
		}

		batch, err := jsc.FetchNoWait(int(op.BatchSize))
		if err != nil {
			return fmt.Errorf("fetch stream batch: %w", err)
		}
		if batch.Error() != nil {
			return fmt.Errorf("batch contains error: %w", batch.Error())
		}
		batchMsg := &commandv1.StreamMessageBatch{}
		for bmsg := range batch.Messages() {
			md, err := bmsg.Metadata()
			if err != nil {
				return fmt.Errorf("get message metadata: %w", err)
			}
			batchMsg.Messages = append(batchMsg.Messages, &commandv1.StreamMessage{
				StreamSequence: md.Sequence.Stream,
				Timestamp:      md.Timestamp.Unix(),
			})
		}
		replyData, err := proto.Marshal(batchMsg)
		if err != nil {
			return fmt.Errorf("marshal stream message batch: %w", err)
		}
		return reply(replyData)
	}

	if op := msg.GetGetStreamMessageContent(); op != nil {
		nc, js, err := connectUser(op)
		if err != nil {
			return err
		}
		defer nc.Close()

		stream, err := js.Stream(ctx, op.StreamName)
		if err != nil {
			return fmt.Errorf("get stream: %w", err)
		}
		rawStreamMsg, err := stream.GetMsg(ctx, op.Sequence)
		if err != nil {
			return fmt.Errorf("get stream message: %w", err)
		}
		replyData, err := proto.Marshal(&commandv1.StreamMessageContent{
			StreamSequence: rawStreamMsg.Sequence,
			Timestamp:      rawStreamMsg.Time.Unix(),
			Data:           rawStreamMsg.Data,
		})
		if err != nil {
			return fmt.Errorf("marshal stream message content: %w", err)
		}
		return reply(replyData)
	}

	if op := msg.GetStartStreamConsumer(); op != nil {
		if op.UserCreds == nil {
			return errors.New("missing user creds for start consumer operation")
		}

		userCreds := op.UserCreds
		consumerID := op.ConsumerId
		err = p.consumerPool.StartConsumer(ctx, op.StreamName, op.StartSequence, consumerID, userCreds.Jwt, userCreds.Seed, func(jsMsg jetstream.Msg, cerr error) {
			// reset context timeout for the consumer message handler
			hctx, hcancel := context.WithTimeout(context.WithoutCancel(ctx), messageHandlerTimeout)
			defer hcancel()

			replyMsg := &commandv1.ReplyMessage{
				Id:    msg.Id,
				Inbox: msg.CommandReplyInbox,
			}
			if jsMsg != nil {
				md, err := jsMsg.Metadata()
				if err != nil {
					p.logger.Error("failed to get consumer message metadata", "error", err, "reply_message.id", replyMsg.Id)
					return
				}
				replyMsg.Id += "_" + strconv.Itoa(int(md.Sequence.Consumer))
				replyMsg.Data, err = proto.Marshal(&commandv1.StreamConsumerMessage{
					StreamSequence:  md.Sequence.Stream,
					MessagesPending: md.NumPending,
					Timestamp:       md.Timestamp.Unix(),
				})
				if err != nil {
					p.logger.Error("failed to marshal consumer message", "error", err, "reply_message.id", replyMsg.Id)
					return
				}
			}
			if cerr != nil {
				errStr := cerr.Error()
				replyMsg.Error = &errStr
			}
			if err := replier(hctx, replyMsg); err != nil {
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

	if op := msg.GetSendStreamConsumerHeartbeat(); op != nil {
		if err := p.consumerPool.SendConsumerHeartbeat(op.ConsumerId); err != nil {
			return err
		}
		return reply(nil)
	}

	if op := msg.GetStopStreamConsumer(); op != nil {
		if err := p.consumerPool.StopConsumer(op.ConsumerId); err != nil && !errtag.HasTag[errtag.NotFound](err) {
			return err
		}
		return reply(nil)
	}

	return errors.New("no operation found in message")
}
