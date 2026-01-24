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

	"github.com/joshjon/kit/errtag"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
)

const (
	accClaimsUpdateSubjectFormat        = "$SYS.REQ.ACCOUNT.%s.CLAIMS.UPDATE"
	accClaimsDeleteSubject              = "$SYS.REQ.CLAIMS.DELETE"
	defaultFetchStreamMessagesBatchSize = 100
)

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
func (c *Commander) NotifyAccountClaimsUpdate(ctx context.Context, account *entity.Account) error {
	claims, err := account.Claims()
	if err != nil {
		return err
	}
	subject := fmt.Sprintf(accClaimsUpdateSubjectFormat, claims.Subject)
	reply, err := c.request(ctx, account.OperatorID, subject, []byte(account.JWT))
	if err != nil {
		return err
	}

	return validateAccountReply(reply.Data)
}

// NotifyAccountClaimsDelete sends a notification with a signed JWT containing
// the account public key(s) to delete.
func (c *Commander) NotifyAccountClaimsDelete(ctx context.Context, operator *entity.Operator, account *entity.Account) error {
	accClaims, err := account.Claims()
	if err != nil {
		return err
	}

	opKP := operator.SigningKey().KeyPair()
	opPK, err := opKP.PublicKey()
	if err != nil {
		return fmt.Errorf("get operator public key: %w", err)
	}

	genericClaims := jwt.NewGenericClaims(opPK)
	genericClaims.Data = map[string]interface{}{
		"accounts": []string{accClaims.Subject},
	}

	delReqJWT, err := genericClaims.Encode(opKP)
	if err != nil {
		return fmt.Errorf("encode delete request JWT: %w", err)
	}

	reply, err := c.request(ctx, account.OperatorID, accClaimsDeleteSubject, []byte(delReqJWT))
	if err != nil {
		return err
	}

	return validateAccountReply(reply.Data)
}

// ListStreams lists all JetStream streams.
func (c *Commander) ListStreams(ctx context.Context, account *entity.Account) ([]*jetstream.StreamInfo, error) {
	result, err, _ := c.sf.Do(account.ID.String(), func() (any, error) {
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
		replyData, err := command(ctx, c.nc, account.OperatorID, msg)
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

// GetStream gets a JetStream streams.
func (c *Commander) GetStream(ctx context.Context, account *entity.Account, streamName string) (*jetstream.StreamInfo, error) {
	sfKey := account.ID.String() + "." + streamName
	result, err, _ := c.sf.Do(sfKey, func() (any, error) {
		user, err := entity.NewUser(fmt.Sprintf("[%s] 'get stream' proxy user", constants.AppNameUpper), account)
		if err != nil {
			return nil, fmt.Errorf("create user for command: %w", err)
		}
		// TODO: update user claims to give least privilege permissions for the operation

		msg := &commandv1.PublishMessage{
			Id:                NewMessageID().String(),
			CommandReplyInbox: nats.NewInbox(),
			Command: &commandv1.PublishMessage_GetStream{
				GetStream: &commandv1.PublishMessage_CommandGetStream{
					StreamName: streamName,
					UserCreds: &commandv1.Credentials{
						Jwt:  user.JWT(),
						Seed: string(user.NKey().Seed),
					},
				},
			},
		}
		replyData, err := command(ctx, c.nc, account.OperatorID, msg)
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

		return unmarshalJSON[*jetstream.StreamInfo](reply.Data)
	})
	if err != nil {
		return nil, err
	}
	return result.(*jetstream.StreamInfo), nil
}

// FetchStreamMessages fetches a batch of messages that are currently available
// in the stream. Does not wait for new messages to arrive, even if batch size
// is not met.
func (c *Commander) FetchStreamMessages(
	ctx context.Context,
	account *entity.Account,
	streamName string,
	startSeq uint64,
	batchSize uint32,
) (*commandv1.StreamMessageBatch, error) {
	user, err := entity.NewUser(fmt.Sprintf("[%s] 'get stream' proxy user", constants.AppNameUpper), account)
	if err != nil {
		return nil, fmt.Errorf("create user for command: %w", err)
	}
	// TODO: update user claims to give least privilege permissions for the operation

	if startSeq == 0 {
		startSeq = 1
	}
	if batchSize == 0 {
		batchSize = defaultFetchStreamMessagesBatchSize
	}

	msg := &commandv1.PublishMessage{
		Id:                NewMessageID().String(),
		CommandReplyInbox: nats.NewInbox(),
		Command: &commandv1.PublishMessage_FetchStreamMessages{
			FetchStreamMessages: &commandv1.PublishMessage_CommandFetchStreamMessages{
				UserCreds: &commandv1.Credentials{
					Jwt:  user.JWT(),
					Seed: string(user.NKey().Seed),
				},
				StreamName:    streamName,
				StartSequence: startSeq,
				BatchSize:     batchSize,
			},
		},
	}
	replyData, err := command(ctx, c.nc, account.OperatorID, msg)
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

	batch := &commandv1.StreamMessageBatch{}
	if err = proto.Unmarshal(reply.Data, batch); err != nil {
		return nil, fmt.Errorf("unmarshal reply message data: %w", err)
	}
	return batch, nil
}

// GetStreamMessageContent gets the content of a message that is currently
// available in the stream.
func (c *Commander) GetStreamMessageContent(
	ctx context.Context,
	account *entity.Account,
	streamName string,
	seq uint64,
) (*commandv1.StreamMessageContent, error) {
	user, err := entity.NewUser(fmt.Sprintf("[%s] 'get stream' proxy user", constants.AppNameUpper), account)
	if err != nil {
		return nil, fmt.Errorf("create user for command: %w", err)
	}
	// TODO: update user claims to give least privilege permissions for the operation

	if seq == 0 {
		return nil, errors.New("sequence number must be greater than 0")
	}

	msg := &commandv1.PublishMessage{
		Id:                NewMessageID().String(),
		CommandReplyInbox: nats.NewInbox(),
		Command: &commandv1.PublishMessage_GetStreamMessageContent{
			GetStreamMessageContent: &commandv1.PublishMessage_CommandGetStreamMessageContent{
				UserCreds: &commandv1.Credentials{
					Jwt:  user.JWT(),
					Seed: string(user.NKey().Seed),
				},
				StreamName: streamName,
				Sequence:   seq,
			},
		},
	}
	replyData, err := command(ctx, c.nc, account.OperatorID, msg)
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

	streamMsg := &commandv1.StreamMessageContent{}
	if err = proto.Unmarshal(reply.Data, streamMsg); err != nil {
		return nil, fmt.Errorf("unmarshal reply message data: %w", err)
	}
	return streamMsg, nil
}

type StreamConsumer interface {
	ID() string
	SendHeartbeat(ctx context.Context) error
	Stop(ctx context.Context) error
}

// ConsumeStream starts an ephemeral consumer on the specified JetStream stream.
func (c *Commander) ConsumeStream(
	account *entity.Account,
	streamName string,
	startSeq uint64,
	handler func(msg *commandv1.ReplyMessage),
) (StreamConsumer, error) {
	user, err := entity.NewUser(fmt.Sprintf("[%s] 'consume stream' proxy user", constants.AppNameUpper), account)
	if err != nil {
		return nil, fmt.Errorf("create user for command: %w", err)
	}
	// TODO: update user claims to give least privilege permissions for the operation
	consumer := newStreamConsumer(c.nc, account.OperatorID, user, streamName)
	if err = consumer.Start(startSeq, handler); err != nil {
		return nil, fmt.Errorf("start stream consumer: %w", err)
	}
	return consumer, nil
}

// Ping checks if the Operator is subscribed to the Broker by sending a ping
// and waiting for a pong reply. Returns true if successful or false if not.
func (c *Commander) Ping(ctx context.Context, operatorID entity.OperatorID) (entity.OperatorNATSStatus, error) {
	result, err, _ := c.sf.Do(operatorID.String(), func() (any, error) {
		brokerPingReplyMsg, err := c.nc.RequestWithContext(ctx, sysServerPingSubject, nil)
		if err != nil {
			return entity.OperatorNATSStatus{}, err
		}
		brokerPingReply, err := unmarshalJSON[pingReplyMessage](brokerPingReplyMsg.Data)
		if err != nil {
			return entity.OperatorNATSStatus{}, err
		}

		numNodes := brokerPingReply.Statsz.ActiveServers

		pingReplyInbox := c.nc.NewInbox()
		sub, err := c.nc.SubscribeSync(pingReplyInbox)
		if err != nil {
			return entity.OperatorNATSStatus{}, fmt.Errorf("subscribe ping operator reply inbox: %w", err)
		}
		defer sub.Unsubscribe() //nolint:errcheck

		subject := getPingOperatorSubject(operatorID)

		if err = c.nc.PublishRequest(subject, pingReplyInbox, nil); err != nil {
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
func (c *Commander) request(ctx context.Context, operatorID entity.OperatorID, subject string, data []byte) (*commandv1.ReplyMessage, error) {
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

	replyData, err := command(ctx, c.nc, operatorID, msg)
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

	buffer := time.Second
	replyCtx, cancel := context.WithTimeout(ctx, messageHandlerTimeout+buffer)
	defer cancel()

	replyNatsMsg, err := replySub.NextMsgWithContext(replyCtx)
	if err != nil {
		if errors.Is(err, nats.ErrNoResponders) {
			return nil, errtag.Tag[errtag.Conflict](err, errtag.WithMsg("Operator NATS not connected"))
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errtag.Tag[errtag.GatewayTimeout](err, errtag.WithMsg("Did not receive a response from Operator NATS"))
		}
		return nil, fmt.Errorf("wait reply message: %w", err)
	}

	reply := &commandv1.ReplyMessage{}
	if err = proto.Unmarshal(replyNatsMsg.Data, reply); err != nil {
		err = fmt.Errorf("unmarshal reply message: %w", err)
		return nil, errtag.Tag[errtag.BadGateway](err, errtag.WithMsg("Received malformed reply from downstream proxy agent"))
	}
	if reply.Error != nil {
		err = fmt.Errorf("error returned in reply message: %s", *reply.Error)
		return nil, errtag.Tag[errtag.BadGateway](err, errtag.WithMsg("Downstream proxy agent failed to handle request"))
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

func validateAccountReply(reply []byte) error {
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
