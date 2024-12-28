package broker

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
	"golang.org/x/time/rate"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/internal/constants"
	"github.com/coro-sh/coro/log"
)

const (
	// versionPath is the base API version path for WebSocket handlers
	versionPath = "/v1"
	// apiKeyHeader is the header key used to pass the API key for authentication
	apiKeyHeader = "X-API-Key"
	// webSocketSubprotocol specifies the WebSocket subprotocol name
	webSocketSubprotocol = constants.AppName + "_broker"

	// Subject formats
	operatorSubjectFormat     = "_" + constants.AppNameUpper + ".BROKER.OPERATOR.%s"
	sysServerPingSubject      = "$SYS.REQ.SERVER.PING"
	pingOperatorSubjectBase   = "_" + constants.AppName + ".BROKER.PING"
	pingOperatorSubjectFormat = pingOperatorSubjectBase + ".OPERATOR.%s"

	// Timeouts and intervals
	writeTimeout      = 30 * time.Second
	heartbeatTimeout  = 10 * time.Second
	heartbeatInterval = 50 * time.Second
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

// Option configures a WebSocketHandler instance.
type Option func(b *WebSocketHandler)

// WithLogger configures a WebSocketHandler to use the specified logger.
func WithLogger(logger log.Logger) Option {
	return func(b *WebSocketHandler) {
		b.logger = logger.With(log.KeyComponent, "broker.websocket_handler")
	}
}

// WebSocketHandler is the WebSocket handler for the Broker server. It receives
// messages via an embedded NATS server and forwards them to corresponding
// WebSocket Operator connections. The WebSocket handler is able to function
// within a cluster of Brokers as long as the embedded NATS server has a
// clustered setup.
type WebSocketHandler struct {
	tknv        TokenVerifier
	entities    EntityReader
	nsSysUser   *entity.User
	ns          *server.Server
	nc          *nats.Conn
	logger      log.Logger
	limiter     *rate.Limiter
	connections sync.Map
}

// NewWebSocketHandler creates a new WebSocketHandler.
func NewWebSocketHandler(
	natsSysUser *entity.User,
	embeddedNats *server.Server,
	tokener TokenVerifier,
	entities EntityReader,
	opts ...Option,
) (*WebSocketHandler, error) {
	h := &WebSocketHandler{
		tknv:      tokener,
		entities:  entities,
		nsSysUser: natsSysUser,
		ns:        embeddedNats,
		logger:    log.NewLogger(),
		// 1 message every 100ms with a 10 message burst
		limiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 10),
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

// Register adds the WebSocketHandler endpoints to the provided Echo router
// group.
func (w *WebSocketHandler) Register(g *echo.Group) {
	v1 := g.Group(versionPath)
	v1.GET("/broker", w.Handle)
}

// Handle first accepts a WebSocket handshake from a client and upgrades the
// connection to a WebSocket. Once connected, the handler subscribes to its
// embedded NATS server for messages intended for the connected Operator and
// forwards them to the WebSocket.
func (w *WebSocketHandler) Handle(c echo.Context) (err error) {
	rctx, rcancel := context.WithCancel(c.Request().Context())
	defer rcancel() // ensures all goroutines are stopped

	logger := w.logger.With("websocket.id", uuid.New().String())

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
	conn, sysUser, err := w.acceptHandshake(rctx, c)
	if err != nil {
		return fmt.Errorf("accept handshake: %w", err)
	}
	getLogger().Info("websocket handshake accepted")
	opID := sysUser.OperatorID
	logger = logger.With(log.KeyOperatorID, opID, log.KeySystemUserID, sysUser.ID)

	w.connections.Store(opID, time.Now())
	defer func() { w.connections.Delete(opID) }()

	const maxWorkerConc = 25
	notifNatsCh := make(chan *nats.Msg, maxWorkerConc)
	wsReplyCh := make(chan Message[json.RawMessage], maxWorkerConc)
	doneCh := make(chan struct{})
	errsCh := make(chan *brokerErr, 4) // aborts handler goroutine with error
	defer func() {
		close(notifNatsCh)
		close(wsReplyCh)
	}()

	// Subscribe to operator notifications in background
	getLogger().Info("subscribing to internal operator notifications")
	subj := getOperatorSubject(opID)
	logger = logger.With("forwarder.subscribe_subject", subj)
	sub, err := w.nc.ChanSubscribe(subj, notifNatsCh)
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
	go func() {
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
					ctx, cancel := context.WithTimeout(rctx, writeTimeout)
					defer cancel()

					metaKV := []any{"nats.subject", fwdMsg.Subject, log.KeyBrokerMessageReplyInbox, fwdMsg.Reply}

					idPayload, werr := unmarshalJSON[messageIDPayload](fwdMsg.Data)
					if werr != nil {
						subWorkerErrsCh <- wrapBrokerErr(fmt.Errorf("writer worker: unmarshal notify message id payload: %w", werr), metaKV...)
						return
					}
					msgID := idPayload.ID

					metaKV = append(metaKV, log.KeyBrokerMessageID, msgID)

					if werr = conn.Write(ctx, websocket.MessageText, fwdMsg.Data); werr != nil {
						subWorkerErrsCh <- wrapBrokerErr(fmt.Errorf("writer worker: write notify message %s to websocket: %w", msgID, werr), metaKV...)
						return
					}

					writeCount.Add(1)
					getLogger().With(metaKV...).Info("published notify message to websocket")
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
	go func() {
		for {
			rerr := w.limiter.Wait(rctx)
			if rerr != nil {
				errsCh <- wrapBrokerErr(fmt.Errorf("reader worker: rate limiter wait: %w", rerr))
				return
			}

			var replyMsg Message[json.RawMessage]
			rerr = wsjson.Read(rctx, conn, &replyMsg)

			if rerr != nil {
				close(doneCh)

				var ce websocket.CloseError
				if errors.As(rerr, &ce) && ce.Code == websocket.StatusNormalClosure {
					getLogger().Info("websocket connection stopped by client", "websocket.close_code", ce.Code, "websocket.close_reason", ce.Reason)
					conn.Close(ce.Code, ce.Reason)
					return
				}

				if errors.Is(rerr, context.Canceled) || errors.Is(rerr, io.EOF) || errors.Is(rerr, net.ErrClosed) {
					code := websocket.StatusGoingAway
					reason := "connection closed"
					conn.Close(code, reason)
					getLogger().Info("websocket connection closed", "websocket.close_code", code, "websocket.close_reason", reason)
				}

				conn.Close(websocket.StatusInternalError, "internal server error")
				errsCh <- wrapBrokerErr(fmt.Errorf("reader worker: read nats notify message reply: %w", rerr))
				return
			}

			readCount.Add(1)
			wsReplyCh <- replyMsg
		}
	}()

	// Start reply worker which publishes replies to their corresponding message inbox
	go func() {
		sem := make(chan struct{}, maxWorkerConc)
		for {
			select {
			case replyMsg, ok := <-wsReplyCh:
				if !ok {
					return
				}
				sem <- struct{}{}

				go func() {
					defer func() { <-sem }()
					metaKV := []any{
						log.KeyBrokerMessageID, replyMsg.ID,
						log.KeyBrokerMessageSubject, replyMsg.Subject,
						log.KeyBrokerMessageReplyInbox, replyMsg.Inbox,
					}

					if perr := w.nc.Publish(replyMsg.Inbox, replyMsg.Data); perr != nil {
						errsCh <- wrapBrokerErr(fmt.Errorf("reply worker: publish notify message nats reply: %w", perr), metaKV...)
						return
					}

					replyCount.Add(1)
					getLogger().With(metaKV...).Info("published notify reply to message inbox")
				}()
			case <-doneCh:
				return
			case <-rctx.Done():
				return
			}
		}
	}()

	// Start heartbeat worker which receives heartbeats from the client
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			case <-doneCh:
				return
			case <-rctx.Done():
				return
			}

			ctx, cancel := context.WithTimeout(rctx, heartbeatTimeout)
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
		case fwdErr := <-errsCh:
			if fwdErr != nil {
				return fwdErr
			}
		case <-rctx.Done():
			return rctx.Err()
		}
	}
}

func (w *WebSocketHandler) acceptHandshake(ctx context.Context, c echo.Context) (*websocket.Conn, *entity.User, error) {
	token := c.Request().Header.Get(apiKeyHeader)
	if token == "" {
		err := errors.New("missing api key header")
		return nil, nil, errtag.Tag[errtag.Unauthorized](err, errtag.WithMsgf("%s header required", apiKeyHeader))
	}

	opID, err := w.tknv.Verify(ctx, token)
	if err != nil {
		return nil, nil, err
	}

	op, err := w.entities.ReadOperator(ctx, opID)
	if err != nil {
		return nil, nil, err
	}

	opClaims, err := op.Claims()
	if err != nil {
		return nil, nil, err
	}

	sysAcc, err := w.entities.ReadAccountByPublicKey(ctx, opClaims.SystemAccount)
	if err != nil {
		return nil, nil, err
	}

	sysUser, err := w.entities.ReadSystemUser(ctx, sysAcc.OperatorID, sysAcc.ID)
	if err != nil {
		return nil, nil, err
	}

	conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		Subprotocols: []string{webSocketSubprotocol},
	})
	if err != nil {
		return nil, nil, err
	}

	if err = wsjson.Write(ctx, conn, SysUserCreds{
		JWT:  sysUser.JWT(),
		Seed: string(sysUser.NKey().Seed),
	}); err != nil {
		return nil, nil, err
	}

	return conn, sysUser, nil
}

func (w *WebSocketHandler) startPingOperatorWorker() error {
	msgs := make(chan *nats.Msg, 5000)
	subj := pingOperatorSubjectBase + ".OPERATOR.>"
	sub, err := w.nc.ChanSubscribe(subj, msgs)
	if err != nil {
		w.nc.Close()
		return fmt.Errorf("nats subscribe internal: %w", err)
	}

	w.logger.Info("started ping operator worker")

	go func() {
		defer w.nc.Close()
		defer sub.Unsubscribe() //nolint:errcheck

		for msg := range msgs {
			opIDStr := strings.TrimPrefix(msg.Subject, pingOperatorSubjectBase+".OPERATOR.")
			logger := w.logger.With(log.KeyOperatorID, opIDStr)

			opID, err := entity.ParseID[entity.OperatorID](opIDStr)
			if err != nil {
				logger.Error("invalid operator id found in ping operator message", "error", err)
				continue
			}

			opStatus := entity.OperatorNATSStatus{Connected: false}
			if v, ok := w.connections.Load(opID); ok {
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

func getOperatorSubject(operatorID entity.OperatorID) string {
	return fmt.Sprintf(operatorSubjectFormat, operatorID)
}

func getPingOperatorSubject(operatorID entity.OperatorID) string {
	return fmt.Sprintf(pingOperatorSubjectFormat, operatorID)
}

func unmarshalJSON[T any](data []byte) (T, error) {
	var t T
	if err := json.Unmarshal(data, &t); err != nil {
		return t, err
	}
	return t, nil
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

func (e *brokerErr) Error() string {
	return e.err.Error()
}
