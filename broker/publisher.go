package broker

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

	"github.com/nats-io/nats.go"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/internal/constants"
)

// PublisherOption configures a Publisher.
type PublisherOption func() (nats.Option, error)

// WithPublisherEmbeddedNATS configures the Publisher to use an embedded NATS
// server connection.
func WithPublisherEmbeddedNATS(natsSrv nats.InProcessConnProvider) PublisherOption {
	return func() (nats.Option, error) {
		return nats.InProcessServer(natsSrv), nil
	}
}

// WithPublisherTLS configures the Publisher with TLS.
func WithPublisherTLS(tlsConfig TLSConfig) PublisherOption {
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

// Publisher connects to the Broker embedded NATS server and publishes messages
// to Operator subjects.
type Publisher struct {
	nc *nats.Conn
}

// DialPublisher creates a Publisher by establishing a connection to the Broker's
// embedded NATS server.
func DialPublisher(brokerNatsURL string, brokerSysUser *entity.User, opts ...PublisherOption) (*Publisher, error) {
	var once sync.Once
	connectedCh := make(chan struct{})

	connectTimeout := 20 * time.Second
	connectAttempts := 10

	natsOpts := []nats.Option{
		nats.Name(constants.AppName + "_broker_publisher"),
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

	return &Publisher{
		nc: nc,
	}, nil
}

// Publish publishes a message to the Operator's NATS subject and waits for a
// reply. A reply is only possible if the Operator is subscribed to the broker,
// which means this method will error if the Operator is not connected.
func (n *Publisher) Publish(ctx context.Context, operatorID entity.OperatorID, subject string, data []byte) ([]byte, error) {
	replyInbox := nats.NewInbox()
	replySub, err := n.nc.SubscribeSync(replyInbox)
	if err != nil {
		return nil, fmt.Errorf("subscribe reply inbox: %w", err)
	}
	defer replySub.Unsubscribe() //nolint:errcheck

	msg := Message[[]byte]{
		ID:      NewMessageID(),
		Inbox:   replyInbox,
		Subject: subject,
		Data:    data,
	}
	msgb, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	if err = n.nc.PublishRequest(getOperatorSubject(operatorID), replyInbox, msgb); err != nil {
		return nil, fmt.Errorf("publish message: %w", err)
	}

	replyMsg, err := replySub.NextMsgWithContext(ctx)
	if err != nil {
		if errors.Is(err, nats.ErrNoResponders) {
			return nil, errtag.Tag[errtag.Conflict](err, errtag.WithMsg("Operator NATS not connected"))
		}
		return nil, fmt.Errorf("wait reply message : %w", err)
	}

	return replyMsg.Data, nil
}

// Ping checks if the Operator is subscribed to the Broker by sending a ping
// and waiting for a pong reply. Returns true if successful or false if not.
func (n *Publisher) Ping(ctx context.Context, operatorID entity.OperatorID) (entity.OperatorNATSStatus, error) {
	brokerPingReplyMsg, err := n.nc.RequestWithContext(ctx, sysServerPingSubject, nil)
	if err != nil {
		return entity.OperatorNATSStatus{}, err
	}
	brokerPingReply, err := unmarshalJSON[brokerPingReplyMessage](brokerPingReplyMsg.Data)
	if err != nil {
		return entity.OperatorNATSStatus{}, err
	}

	numNodes := brokerPingReply.Statsz.ActiveServers

	pingReplyInbox := n.nc.NewInbox()
	sub, err := n.nc.SubscribeSync(pingReplyInbox)
	if err != nil {
		return entity.OperatorNATSStatus{}, fmt.Errorf("subscribe ping operator reply inbox: %w", err)
	}
	defer sub.Unsubscribe() //nolint:errcheck

	subject := getPingOperatorSubject(operatorID)

	if err = n.nc.PublishRequest(subject, pingReplyInbox, nil); err != nil {
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
