package command

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"golang.org/x/sync/singleflight"
	"google.golang.org/protobuf/proto"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/internal/constants"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
)

const accClaimsUpdateSubjectFormat = "$SYS.REQ.ACCOUNT.%s.CLAIMS.UPDATE"

// CommanderOption configures a Commander.
type CommanderOption func() (nats.Option, error)

// WithCommanderEmbeddedNATS configures the Commander to use an embedded NATS
// server connection.
func WithCommanderEmbeddedNATS(natsSrv nats.InProcessConnProvider) CommanderOption {
	return func() (nats.Option, error) {
		return nats.InProcessServer(natsSrv), nil
	}
}

// WithCommanderTLS configures the Commander with TLS.
func WithCommanderTLS(tlsConfig TLSConfig) CommanderOption {
	return func() (nats.Option, error) {
		cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load nats tls certificates: %w", err)
		}

		natsTLSCfg := &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
		}

		if tlsConfig.CACertFile != "" {
			natsTLSCfg.RootCAs, err = loadCACert(tlsConfig.CACertFile)
			if err != nil {
				return nil, err
			}
		}

		return nats.Secure(natsTLSCfg), nil
	}
}

// Commander connects to the Broker embedded NATS server and publishes messages
// to Operator subjects.
type Commander struct {
	nc *nats.Conn
	sf singleflight.Group
}

// NewCommander creates a Commander by establishing a connection to the Broker's
// embedded NATS server.
func NewCommander(brokerNatsURL string, brokerSysUser *entity.User, opts ...CommanderOption) (*Commander, error) {
	var once sync.Once
	connectedCh := make(chan struct{})

	connectTimeout := 20 * time.Second
	connectAttempts := 10

	natsOpts := []nats.Option{
		nats.Name(constants.AppName + "_broker_commander"),
		nats.UserJWTAndSeed(brokerSysUser.JWT(), string(brokerSysUser.NKey().Seed)),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(connectAttempts),
		nats.CustomReconnectDelay(func(_ int) time.Duration {
			return connectTimeout / time.Duration(connectAttempts)
		}),
		nats.ConnectHandler(func(_ *nats.Conn) {
			once.Do(func() { close(connectedCh) })
		}),
	}

	for _, opt := range opts {
		natsOpt, err := opt()
		if err != nil {
			return nil, err
		}
		natsOpts = append(natsOpts, natsOpt)
	}

	nc, err := nats.Connect(brokerNatsURL, natsOpts...)
	if err != nil {
		return nil, err
	}

	select {
	case <-connectedCh:
	case <-time.After(connectTimeout):
		nc.Close()
		return nil, errors.New("broker nats connect timeout")
	}

	return &Commander{
		nc: nc,
	}, nil
}

// NotifyAccountClaimsUpdate sends a notification about an account claims update.
func (p *Commander) NotifyAccountClaimsUpdate(ctx context.Context, account *entity.Account) error {
	claims, err := account.Claims()
	if err != nil {
		return err
	}
	subject := fmt.Sprintf(accClaimsUpdateSubjectFormat, claims.Subject)
	reply, err := p.request(ctx, account.OperatorID, subject, []byte(account.JWT))
	if err != nil {
		return err
	}

	return validateAccountReply(reply.Data, claims, "jwt updated")
}

// ListStreams lists all JetStream streams.
func (p *Commander) ListStreams(ctx context.Context, account *entity.Account) ([]*jetstream.StreamInfo, error) {
	result, err, _ := p.sf.Do(account.ID.String(), func() (any, error) {
		user, err := entity.NewUser(fmt.Sprintf("[%s] 'list streams' proxy user", constants.AppNameUpper), account)
		if err != nil {
			return nil, fmt.Errorf("create user for command: %w", err)
		}
		// TODO: update user claims to give least privilege permissions for the operation

		msg := &commandv1.PublishMessage{
			Id:                NewMessageID().String(),
			CommandReplyInbox: nats.NewInbox(),
			Command: &commandv1.PublishMessage_ListStream{
				ListStream: &commandv1.PublishMessage_CommandListStreams{
					UserCreds: &commandv1.Credentials{
						Jwt:  user.JWT(),
						Seed: string(user.NKey().Seed),
					},
				},
			},
		}
		replyData, err := command(ctx, p.nc, account.OperatorID, msg)
		if err != nil {
			return nil, err
		}

		reply := &commandv1.ReplyMessage{}
		if err = proto.Unmarshal(replyData, reply); err != nil {
			return nil, fmt.Errorf("unmarshal reply message: %w", err)
		}

		if reply.Error != nil {
			return nil, fmt.Errorf("error returned in reply message: %s", *reply.Error)
		}

		return unmarshalJSON[[]*jetstream.StreamInfo](reply.Data)
	})
	if err != nil {
		return nil, err
	}
	return result.([]*jetstream.StreamInfo), nil
}

type StreamConsumer interface {
	ID() string
	SendHeartbeat(ctx context.Context) error
	Stop(ctx context.Context) error
}

// ConsumeStream starts an ephemeral consumer on the specified JetStream stream.
func (p *Commander) ConsumeStream(
	account *entity.Account,
	streamName string,
	handler func(msg *commandv1.ReplyMessage),
) (StreamConsumer, error) {
	user, err := entity.NewUser(fmt.Sprintf("[%s] 'consume stream' proxy user", constants.AppNameUpper), account)
	if err != nil {
		return nil, fmt.Errorf("create user for command: %w", err)
	}
	// TODO: update user claims to give least privilege permissions for the operation
	consumer := newStreamConsumer(p.nc, account.OperatorID, user, streamName)
	if err = consumer.Start(handler); err != nil {
		return nil, fmt.Errorf("start stream consumer: %w", err)
	}
	return consumer, nil
}

// Ping checks if the Operator is subscribed to the Broker by sending a ping
// and waiting for a pong reply. Returns true if successful or false if not.
func (p *Commander) Ping(ctx context.Context, operatorID entity.OperatorID) (entity.OperatorNATSStatus, error) {
	result, err, _ := p.sf.Do(operatorID.String(), func() (any, error) {
		brokerPingReplyMsg, err := p.nc.RequestWithContext(ctx, sysServerPingSubject, nil)
		if err != nil {
			return entity.OperatorNATSStatus{}, err
		}
		brokerPingReply, err := unmarshalJSON[pingReplyMessage](brokerPingReplyMsg.Data)
		if err != nil {
			return entity.OperatorNATSStatus{}, err
		}

		numNodes := brokerPingReply.Statsz.ActiveServers

		pingReplyInbox := p.nc.NewInbox()
		sub, err := p.nc.SubscribeSync(pingReplyInbox)
		if err != nil {
			return entity.OperatorNATSStatus{}, fmt.Errorf("subscribe ping operator reply inbox: %w", err)
		}
		defer sub.Unsubscribe() //nolint:errcheck

		subject := getPingOperatorSubject(operatorID)

		if err = p.nc.PublishRequest(subject, pingReplyInbox, nil); err != nil {
			return entity.OperatorNATSStatus{}, fmt.Errorf("publish ping operator message: %w", err)
		}

		for range numNodes {
			replyMsg, err := sub.NextMsgWithContext(ctx)
			if err != nil {
				return entity.OperatorNATSStatus{}, fmt.Errorf("receive ping operator inbox reply: %w", err)
			}
			opStatus, err := unmarshalJSON[entity.OperatorNATSStatus](replyMsg.Data)
			if err != nil {
				return entity.OperatorNATSStatus{}, err
			}
			if !opStatus.Connected {
				continue
			}
			return opStatus, nil
		}

		return entity.OperatorNATSStatus{
			Connected: false,
		}, nil
	})
	if err != nil {
		return entity.OperatorNATSStatus{}, err
	}
	return result.(entity.OperatorNATSStatus), nil
}

// request publishes a message to the Operator's NATS subject and waits
// for a reply. A reply is only possible if the Operator is subscribed to the
// broker, which means this method will error if the Operator is not connected.
func (p *Commander) request(ctx context.Context, operatorID entity.OperatorID, subject string, data []byte) (*commandv1.ReplyMessage, error) {
	msg := &commandv1.PublishMessage{
		Id:                NewMessageID().String(),
		CommandReplyInbox: nats.NewInbox(),
		Command: &commandv1.PublishMessage_Request{
			Request: &commandv1.PublishMessage_CommandRequest{
				Subject: subject,
				Data:    data,
			},
		},
	}

	replyData, err := command(ctx, p.nc, operatorID, msg)
	if err != nil {
		return nil, err
	}

	replypb := &commandv1.ReplyMessage{}
	if err = proto.Unmarshal(replyData, replypb); err != nil {
		return nil, fmt.Errorf("unmarshal reply message: %w", err)
	}

	if replypb.Error != nil {
		return nil, fmt.Errorf("error returned in reply message: %s", *replypb.Error)
	}

	return replypb, nil
}

func command(ctx context.Context, nc *nats.Conn, operatorID entity.OperatorID, msg *commandv1.PublishMessage) ([]byte, error) {
	replySub, err := nc.SubscribeSync(msg.CommandReplyInbox)
	if err != nil {
		return nil, fmt.Errorf("subscribe reply inbox: %w", err)
	}
	defer replySub.Unsubscribe() //nolint:errcheck

	msgb, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	if err = nc.PublishRequest(getOperatorSubject(operatorID), msg.CommandReplyInbox, msgb); err != nil {
		return nil, fmt.Errorf("publish message: %w", err)
	}

	replyNatsMsg, err := replySub.NextMsgWithContext(ctx)
	if err != nil {
		if errors.Is(err, nats.ErrNoResponders) {
			return nil, errtag.Tag[errtag.Conflict](err, errtag.WithMsg("Operator NATS not connected"))
		}
		return nil, fmt.Errorf("wait reply message : %w", err)
	}

	return replyNatsMsg.Data, nil
}

func loadCACert(caCertFile string) (*x509.CertPool, error) {
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("read ca certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to append ca certificate")
	}

	return caCertPool, nil
}

func validateAccountReply(reply []byte, claims *jwt.AccountClaims, wantMsg string) error {
	if len(reply) == 0 {
		return errors.New("empty account reply")
	}

	var replyMsg accountUpdateReplyMessage
	if err := json.Unmarshal(reply, &replyMsg); err != nil {
		return fmt.Errorf("unmarshal account reply message: %w", err)
	}

	data := replyMsg.Data

	switch {
	case data.Code < 200 && data.Code > 299:
		return fmt.Errorf("non 200 status code: %d", data.Code)
	case claims.Subject != data.Account:
		return errors.New("claims subject mismatch")
	case data.Message != wantMsg:
		return fmt.Errorf("expected reply '%s', got '%s'", wantMsg, data.Message)
	}

	return nil
}

func unmarshalJSON[T any](data []byte) (T, error) {
	var t T
	if err := json.Unmarshal(data, &t); err != nil {
		return t, err
	}
	return t, nil
}
