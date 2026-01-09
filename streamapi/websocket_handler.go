package streamapi

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/joshjon/kit/id"
	"github.com/joshjon/kit/log"
	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"

	"github.com/coro-sh/coro/command"
	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/entityapi"
	"github.com/coro-sh/coro/logkey"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
	"github.com/coro-sh/coro/websocketutil"
)

// streamWebSocketSubprotocol specifies the WebSocket subprotocol name.
const (
	streamWebSocketSubprotocol = constants.AppName + "_stream"
	consumerHeartbeatInterval  = command.MaxConsumerIdleHeartbeat / 2
)

type ConsumerStarter interface {
	ConsumeStream(account *entity.Account, streamName string, startSeq uint64, handler func(msg *commandv1.ReplyMessage)) (command.StreamConsumer, error)
}

type StreamWebSocketHandlerOption func(h *StreamWebSocketHandler)

// WithStreamWebSocketHandlerCORS configures the server to authorize Cross-Origin
// Resource Sharing (CORS) for the provided origins.
func WithStreamWebSocketHandlerCORS(origins ...string) StreamWebSocketHandlerOption {
	return func(s *StreamWebSocketHandler) {
		s.corsOrigins = origins
	}
}

// StreamWebSocketHandler handles stream related WebSocket requests.
type StreamWebSocketHandler struct {
	accReader         AccountReader
	consumerStarter   ConsumerStarter
	heartbeatInterval time.Duration
	logger            log.Logger
	numConns          atomic.Int64
	corsOrigins       []string
}

// NewStreamWebSocketHandler creates a new StreamWebSocketHandler.
func NewStreamWebSocketHandler(accounts AccountReader, consumerStarter ConsumerStarter, opts ...StreamWebSocketHandlerOption) *StreamWebSocketHandler {
	s := &StreamWebSocketHandler{
		accReader:         accounts,
		consumerStarter:   consumerStarter,
		heartbeatInterval: consumerHeartbeatInterval,
		logger:            log.NewLogger(),
		numConns:          atomic.Int64{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *StreamWebSocketHandler) Register(g *echo.Group) {
	account := g.Group(fmt.Sprintf("/namespaces/:%s/accounts/:%s", entityapi.PathParamNamespaceID, entityapi.PathParamAccountID))
	account.GET(fmt.Sprintf("/streams/:%s/consume", PathParamStreamName), s.HandleConsume)
}

func (s *StreamWebSocketHandler) NumConnections() int64 {
	return s.numConns.Load()
}

func (s *StreamWebSocketHandler) HandleConsume(c echo.Context) (err error) {
	ctx := c.Request().Context()
	logger := s.logger.With("websocket.id", uuid.New().String())
	wg := new(sync.WaitGroup)

	req, err := server.BindRequest[StartStreamConsumerRequest](c)
	if err != nil {
		return err
	}
	accID := id.MustParse[entity.AccountID](req.AccountID)

	c.Set(logkey.AccountID, accID)
	c.Set(logkey.StreamName, req.StreamName)
	c.Set(logkey.ConsumerStartSequence, req.StartSequence)

	logger = logger.With(
		logkey.AccountID, accID,
		logkey.StreamName, req.StreamName,
		logkey.ConsumerStartSequence, req.StartSequence,
	)

	acc, err := s.accReader.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}
	if err = entityapi.VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	if req.StartSequence == 0 {
		req.StartSequence = 1
	}

	conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		Subprotocols:   []string{streamWebSocketSubprotocol},
		OriginPatterns: s.corsOrigins,
	})
	if err != nil {
		return err
	}
	s.numConns.Add(1)

	var errMsg = "internal server error"
	var errDetails []string

	defer func() {
		code, reason := websocketutil.GetCloseErrCodeAndReason(err)
		if code == websocket.StatusNormalClosure {
			err = nil // not an actual failure
		}
		if err != nil {
			if werr := wsjson.Write(ctx, conn, server.ResponseError{
				Error: server.HTTPError{
					Message: errMsg,
					Details: errDetails,
				},
			}); werr != nil {
				logger.Error("failed to write error message", "error", werr)
			}
		}
		websocketutil.CloseConn(conn, code, reason, logger)
		wg.Wait()
		s.numConns.Add(-1)
	}()

	errCh := make(chan error, 2)

	isStarted := false
	isStartedCh := make(chan struct{})

	consumer, err := s.consumerStarter.ConsumeStream(acc, req.StreamName, req.StartSequence, func(msg *commandv1.ReplyMessage) {
		if !isStarted {
			// first message will always have empty reply to indicate consumer successfully created
			if msg.Data == nil && msg.Error == nil {
				isStarted = true
				close(isStartedCh)
				return
			}
			errCh <- fmt.Errorf("received unexpected message: expected consumer to start acknowledgement")
			return
		}
		cmsg := &commandv1.StreamConsumerMessage{}
		if err := proto.Unmarshal(msg.Data, cmsg); err != nil {
			errCh <- fmt.Errorf("unmarshal consumer message: %w", err)
			return
		}
		if werr := wsjson.Write(ctx, conn, server.Response[*commandv1.StreamConsumerMessage]{
			Data: cmsg,
		}); werr != nil {
			errCh <- werr
		}
	})
	if err != nil {
		return err
	}
	defer func() {
		if cerr := consumer.Stop(ctx); cerr != nil {
			logger.Error("failed to stop consumer for websocket", "error", cerr)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// keep a read loop open to ensure the client has not closed the connection
		// and to allow automatic handling of pong replies.
		for {
			_, _, rerr := conn.Read(ctx)
			if rerr != nil {
				errCh <- rerr
				return
			}
		}
	}()

	// wait for consumer to start
	select {
	case <-isStartedCh:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for consumer to start")
	case err = <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}

	// heartbeat loop
	for {
		select {
		case <-ctx.Done():
			return nil
		case err = <-errCh:
			return err
		case <-time.After(s.heartbeatInterval):
			if err = conn.Ping(ctx); err != nil {
				return errors.New("client did not respond to ping")
			}
			if err = consumer.SendHeartbeat(ctx); err != nil {
				return fmt.Errorf("send consumer heartbeat: %w", err)
			}
		}
	}
}
