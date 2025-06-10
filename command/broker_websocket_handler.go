package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/log"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
)

const (
	// apiKeyHeader is the header key used to pass the API key for authentication
	apiKeyHeader = "X-API-Key"
	// brokerWebSocketSubprotocol specifies the WebSocket subprotocol name
	brokerWebSocketSubprotocol = constants.AppName + "_broker"

	// Subject formats
	operatorSubjectFormat     = "_" + constants.AppNameUpper + ".BROKER.OPERATOR.%s"
	sysServerPingSubject      = "$SYS.REQ.SERVER.PING"
	pingOperatorSubjectBase   = "_" + constants.AppName + ".BROKER.PING"
	pingOperatorSubjectFormat = pingOperatorSubjectBase + ".OPERATOR.%s"

	// Timeouts and intervals
	brokerWriteTimeout      = 30 * time.Second
	brokerHeartbeatTimeout  = 10 * time.Second
	brokerHeartbeatInterval = 50 * time.Second
)

// TokenVerifier verifies API tokens and extracts their associated Operator ID.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (entity.OperatorID, error)
}

// EntityReader reads entities from storage.
type EntityReader interface {
	ReadOperator(ctx context.Context, id entity.OperatorID) (*entity.Operator, error)
	ReadAccountByPublicKey(ctx context.Context, pubKey string) (*entity.Account, error)
	ReadSystemUser(ctx context.Context, operatorID entity.OperatorID, accountID entity.AccountID) (*entity.User, error)
}

// BrokerWebsocketOption configures a BrokerWebSocketHandler instance.
type BrokerWebsocketOption func(b *BrokerWebSocketHandler)

// WithBrokerWebsocketLogger configures a BrokerWebSocketHandler to use the specified logger.
func WithBrokerWebsocketLogger(logger log.Logger) BrokerWebsocketOption {
	return func(b *BrokerWebSocketHandler) {
		b.logger = logger.With(log.KeyComponent, "broker.websocket_handler")
	}
}

// BrokerWebSocketHandler is the WebSocket handler for the Broker server.
// It receives messages via an embedded NATS server and forwards them to
// corresponding WebSocket Operator connections. The WebSocket handler is able
// to function within a cluster of Brokers as long as the embedded NATS server
// has a clustered setup.
type BrokerWebSocketHandler struct {
	tknv        TokenVerifier
	entities    EntityReader
	nsSysUser   *entity.User
	ns          *server.Server
	nc          *nats.Conn
	logger      log.Logger
	connections *sync.Map
	numConns    atomic.Int64
}

// NewBrokerWebSocketHandler creates a new BrokerWebSocketHandler.
func NewBrokerWebSocketHandler(
	natsSysUser *entity.User,
	embeddedNats *server.Server,
	tokener TokenVerifier,
	entities EntityReader,
	opts ...BrokerWebsocketOption,
) (*BrokerWebSocketHandler, error) {
	h := &BrokerWebSocketHandler{
		tknv:        tokener,
		entities:    entities,
		nsSysUser:   natsSysUser,
		ns:          embeddedNats,
		logger:      log.NewLogger(),
		connections: new(sync.Map),
		numConns:    atomic.Int64{},
	}
	for _, opt := range opts {
		opt(h)
	}
	if !h.ns.ReadyForConnections(10 * time.Second) {
		return nil, errors.New("embedded nats server not healthy")
	}

	nc, err := nats.Connect("",
		nats.InProcessServer(h.ns),
		nats.UserJWTAndSeed(h.nsSysUser.JWT(), string(h.nsSysUser.NKey().Seed)),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect in process server: %w", err)
	}
	h.nc = nc

	if err := h.startPingOperatorWorker(); err != nil {
		return nil, err
	}

	return h, nil
}

// Register adds the BrokerWebSocketHandler endpoints to the provided Echo router
// group.
func (b *BrokerWebSocketHandler) Register(g *echo.Group) {
	g.GET("/broker", b.Handle)
}

func (b *BrokerWebSocketHandler) NumConnections() int64 {
	return b.numConns.Load()
}

// Handle first accepts a WebSocket handshake from a client and upgrades the
// connection to a WebSocket. Once connected, the handler subscribes to its
// embedded NATS server for messages intended for the connected Operator and
// forwards them to the WebSocket.
func (b *BrokerWebSocketHandler) Handle(c echo.Context) (err error) {
	rctx, rcancel := context.WithCancel(c.Request().Context())
	defer rcancel() // ensures all goroutines are stopped

	logger := b.logger.With("websocket.id", uuid.New().String())

	var logCallbacks []func(l log.Logger) log.Logger
	getLogger := func() log.Logger {
		l := logger
		for _, cb := range logCallbacks {
			if cb != nil {
				l = cb(l)
			}
		}
		return l
	}

	defer func() {
		if err != nil {
			errLogger := getLogger()
			var berr *brokerErr
			if errors.As(err, &berr) {
				errLogger = errLogger.With(berr.keyVals...)
			}
			errLogger.Error("websocket failed", "error", err)
			return
		}
		getLogger().Info("websocket finished")
	}()

	getLogger().Info("accepting websocket handshake")
	conn, sysUser, err := b.acceptHandshake(rctx, c)
	if err != nil {
		return fmt.Errorf("accept handshake: %w", err)
	}

	const maxWorkerConc = 25
	notifNatsCh := make(chan *nats.Msg, maxWorkerConc)
	wsReplyCh := make(chan *commandv1.ReplyMessage, maxWorkerConc)
	errsCh := make(chan *brokerErr, 4) // aborts worker goroutine with error
	doneCh := make(chan struct{})      // stops all worker goroutines
	wg := new(sync.WaitGroup)          // wait for all worker goroutine to finish

	b.numConns.Add(1)
	defer func() {
		close(notifNatsCh)
		close(wsReplyCh)
		close(doneCh)
		// close the websocket conn first
		code, reason := getWebSocketCloseCodeAndReason(err)
		if code == websocket.StatusNormalClosure {
			err = nil // not an actual failure
		}
		closeWebSocketConn(conn, logger, code, reason)
		b.numConns.Add(-1)
		// then wait for all goroutines to finish
		wg.Wait()
	}()

	getLogger().Info("websocket handshake accepted")
	opID := sysUser.OperatorID
	logger = logger.With(log.KeyOperatorID, opID, log.KeySystemUserID, sysUser.ID)

	b.connections.Store(opID, time.Now())
	defer func() { b.connections.Delete(opID) }()

	// Subscribe to operator notifications in background
	getLogger().Info("subscribing to internal operator notifications")
	subj := getOperatorSubject(opID)
	logger = logger.With("forwarder.subscribe_subject", subj)
	sub, err := b.nc.ChanSubscribe(subj, notifNatsCh)
	if err != nil {
		return fmt.Errorf("nats subscribe operator notifications: %w", err)
	}
	defer sub.Unsubscribe() //nolint:errcheck

	start := time.Now()
	writeCount := new(atomic.Int64)
	readCount := new(atomic.Int64)
	replyCount := new(atomic.Int64)
	hearbeatCount := new(atomic.Int64)

	logCallbacks = append(logCallbacks, func(l log.Logger) log.Logger {
		return l.With(
			"websocket.duration_ms", time.Since(start).Milliseconds(),
			"websocket.duration_human", time.Since(start).String(),
			"websocket.write_count", writeCount.Load(),
			"websocket.read_count", readCount.Load(),
			"websocket.heartbeat_count", hearbeatCount.Load(),
			"forwarder.inbox_reply_count", readCount.Load(),
		)
	})

	// Start writer worker that forwards notifications to the client
	wg.Add(1)
	go func() {
		defer wg.Done()
		sem := make(chan struct{}, maxWorkerConc)
		subWorkerErrsCh := make(chan *brokerErr, maxWorkerConc)
		for {
			select {
			case msg, ok := <-notifNatsCh:
				if !ok {
					return
				}
				sem <- struct{}{}

				go func(fwdMsg *nats.Msg) {
					defer func() { <-sem }()
					ctx, cancel := context.WithTimeout(rctx, brokerWriteTimeout)
					defer cancel()

					metaKV := []any{"nats.subject", fwdMsg.Subject, log.KeyBrokerMessageReplyInbox, fwdMsg.Reply}

					// Unmarshal the message so we can log the message ID
					msgpb := &commandv1.PublishMessage{}
					if werr := proto.Unmarshal(fwdMsg.Data, msgpb); werr != nil {
						subWorkerErrsCh <- wrapBrokerErr(fmt.Errorf("writer worker: unmarshal forward message: %w", werr), metaKV...)
						return
					}
					msgID := msgpb.Id

					metaKV = append(metaKV, log.KeyBrokerMessageID, msgpb.Id)

					if werr := conn.Write(ctx, websocket.MessageText, fwdMsg.Data); werr != nil {
						subWorkerErrsCh <- wrapBrokerErr(fmt.Errorf("writer worker: write notify message %s to websocket: %w", msgID, werr), metaKV...)
						return
					}

					writeCount.Add(1)
					getLogger().With(metaKV...).Info("published command message to websocket")
				}(msg)
			case werr := <-subWorkerErrsCh:
				if werr != nil {
					errsCh <- werr
				}
			case <-doneCh:
				return
			case <-rctx.Done():
				return
			}
		}
	}()

	// Start reader worker that reads notification replies from the client
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, msgb, rerr := conn.Read(rctx)
			if rerr != nil {
				errsCh <- wrapBrokerErr(fmt.Errorf("reader worker: read nats message reply: %w", rerr))
				return
			}

			replyMsg := &commandv1.ReplyMessage{}
			if rerr = proto.Unmarshal(msgb, replyMsg); rerr != nil {
				errsCh <- wrapBrokerErr(fmt.Errorf("reader worker: unmarshal reply message: %w", rerr))
				continue
			}

			readCount.Add(1)
			wsReplyCh <- replyMsg
		}
	}()

	// Start reply worker which publishes replies to their corresponding message inbox
	wg.Add(1)
	go func() {
		defer wg.Done()
		mu := new(sync.Mutex)
		replyInboxChans := make(map[string]chan *commandv1.ReplyMessage)

		sem := make(chan struct{}, maxWorkerConc)
		for {
			select {
			case replyMsg, ok := <-wsReplyCh:
				if !ok {
					return
				}

				// Each message published to a reply inbox is done within a new worker goroutine
				// after acquiring access to the semaphore. However, some reply inboxes may also
				// expect many messages to be published (e.g. a stream consumer). To ensure,
				// messages remain ordered when publishing to the same reply inbox, we maintain
				// short-lived channels in a map (synchronized via a mutex), which act as a queue
				// for publishing to that inbox. So if an inbox is currently being published to,
				// then any other messages received for the same inbox **during this period** will
				// be forwarded to the channel. This guarantees replies will be published in order,
				// since we will never end up having two separate goroutines for the same inbox at
				// any given time. Finally, if there are no active messages within the channel
				// buffer to publish, then the channel is removed.
				mu.Lock()
				if _, ok := replyInboxChans[replyMsg.Inbox]; ok {
					replyInboxChans[replyMsg.Inbox] <- replyMsg
					mu.Unlock()
				} else {
					sem <- struct{}{}
					replyInboxCh := make(chan *commandv1.ReplyMessage, 10)
					replyInboxCh <- replyMsg
					replyInboxChans[replyMsg.Inbox] = replyInboxCh

					go func() {
						defer func() {
							delete(replyInboxChans, replyMsg.Inbox)
							mu.Unlock()
							<-sem
						}()
						for {
							select {
							case nextReplyMsg := <-replyInboxCh:
								metaKV := []any{
									log.KeyBrokerMessageID, nextReplyMsg.Id,
									log.KeyBrokerMessageReplyInbox, nextReplyMsg.Inbox,
								}

								msgb, err := proto.Marshal(nextReplyMsg)
								if err != nil {
									errsCh <- wrapBrokerErr(fmt.Errorf("reply worker: marshal reply message: %w", err), metaKV...)
									return
								}

								if perr := b.nc.Publish(nextReplyMsg.Inbox, msgb); perr != nil {
									errsCh <- wrapBrokerErr(fmt.Errorf("reply worker: publish notify message nats reply: %w", perr), metaKV...)
									return
								}

								replyCount.Add(1)
								getLogger().With(metaKV...).Info("published notify reply to message inbox")
							default:
								// no more pending messages for inbox
								return
							}
						}
					}()
				}
			case <-doneCh:
				return
			case <-rctx.Done():
				return
			}
		}
	}()

	// Start heartbeat worker which receives heartbeats from the client
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(brokerHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			case <-doneCh:
				return
			case <-rctx.Done():
				return
			}

			ctx, cancel := context.WithTimeout(rctx, brokerHeartbeatTimeout)
			perr := conn.Ping(ctx)
			cancel()
			if err != nil {
				errsCh <- wrapBrokerErr(fmt.Errorf("heartbeat worker: ping websocket client: %w", perr))
				return
			}
			hearbeatCount.Add(1)
		}
	}()

	for {
		select {
		case <-doneCh:
			getLogger().Info("client closed notify websocket connection")
			return nil
		case err = <-errsCh:
			if err != nil {
				return err
			}
		case <-rctx.Done():
			return rctx.Err()
		}
	}
}

func (b *BrokerWebSocketHandler) acceptHandshake(ctx context.Context, c echo.Context) (*websocket.Conn, *entity.User, error) {
	token := c.Request().Header.Get(apiKeyHeader)
	if token == "" {
		err := errors.New("missing api key header")
		return nil, nil, errtag.Tag[errtag.Unauthorized](err, errtag.WithMsgf("%s header required", apiKeyHeader))
	}

	opID, err := b.tknv.Verify(ctx, token)
	if err != nil {
		return nil, nil, err
	}

	op, err := b.entities.ReadOperator(ctx, opID)
	if err != nil {
		return nil, nil, err
	}

	opClaims, err := op.Claims()
	if err != nil {
		return nil, nil, err
	}

	sysAcc, err := b.entities.ReadAccountByPublicKey(ctx, opClaims.SystemAccount)
	if err != nil {
		return nil, nil, err
	}

	sysUser, err := b.entities.ReadSystemUser(ctx, sysAcc.OperatorID, sysAcc.ID)
	if err != nil {
		return nil, nil, err
	}

	conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		Subprotocols:       []string{brokerWebSocketSubprotocol},
		InsecureSkipVerify: true, // allow all origins since this is a public websocket
	})
	if err != nil {
		return nil, nil, err
	}

	if err = wsjson.Write(ctx, conn, UserCreds{
		JWT:  sysUser.JWT(),
		Seed: string(sysUser.NKey().Seed),
	}); err != nil {
		return nil, nil, err
	}

	return conn, sysUser, nil
}

func (b *BrokerWebSocketHandler) startPingOperatorWorker() error {
	msgs := make(chan *nats.Msg, 5000)
	subj := pingOperatorSubjectBase + ".OPERATOR.>"
	sub, err := b.nc.ChanSubscribe(subj, msgs)
	if err != nil {
		b.nc.Close()
		return fmt.Errorf("nats subscribe internal: %w", err)
	}

	b.logger.Info("started ping operator worker")

	go func() {
		defer b.nc.Close()
		defer sub.Unsubscribe() //nolint:errcheck

		for msg := range msgs {
			opIDStr := strings.TrimPrefix(msg.Subject, pingOperatorSubjectBase+".OPERATOR.")
			logger := b.logger.With(log.KeyOperatorID, opIDStr)

			opID, err := entity.ParseID[entity.OperatorID](opIDStr)
			if err != nil {
				logger.Error("invalid operator id found in ping operator message", "error", err)
				continue
			}

			opStatus := entity.OperatorNATSStatus{Connected: false}
			if v, ok := b.connections.Load(opID); ok {
				opStatus.Connected = true
				connectTime := v.(time.Time).Unix()
				opStatus.ConnectTime = &connectTime
			}
			replyData, err := json.Marshal(opStatus)
			if err != nil {
				logger.Error("failed to marshal operator status", "error", err)
			}
			if err = msg.Respond(replyData); err != nil {
				logger.Error("failed to respond to ping operator message", "error", err)
				continue
			}
		}
	}()

	return nil
}

func closeWebSocketConn(conn *websocket.Conn, logger log.Logger, code websocket.StatusCode, reason string) {
	if cerr := conn.Close(code, reason); cerr != nil && !errors.Is(cerr, net.ErrClosed) {
		logger.Error("failed to close websocket connection", "error", cerr, "websocket.close_code", code, "websocket.close_reason", reason)
		return
	}
	logger.Info("websocket connection closed", "websocket.close_code", code, "websocket.close_reason", reason)
}

func getWebSocketCloseCodeAndReason(err error) (websocket.StatusCode, string) {
	if err != nil {
		var ce websocket.CloseError
		if errors.As(err, &ce) && ce.Code == websocket.StatusNormalClosure {
			return ce.Code, ce.Reason
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			return websocket.StatusGoingAway, "connection closed"
		}

		return websocket.StatusInternalError, "internal server error"
	}
	return websocket.StatusNormalClosure, ""
}

func getOperatorSubject(operatorID entity.OperatorID) string {
	return fmt.Sprintf(operatorSubjectFormat, operatorID)
}

func getPingOperatorSubject(operatorID entity.OperatorID) string {
	return fmt.Sprintf(pingOperatorSubjectFormat, operatorID)
}

type brokerErr struct {
	err     error
	keyVals []any
}

func wrapBrokerErr(err error, keyVals ...any) *brokerErr {
	return &brokerErr{
		err:     err,
		keyVals: keyVals,
	}
}

func (b *brokerErr) Unwrap() error {
	return b.err
}

func (b *brokerErr) Error() string {
	return b.err.Error()
}
