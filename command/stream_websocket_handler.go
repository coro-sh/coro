package command

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/cohesivestack/valgo"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/internal/constants"
	"github.com/coro-sh/coro/internal/valgoutil"
	"github.com/coro-sh/coro/log"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
	"github.com/coro-sh/coro/server"
)

// streamWebSocketSubprotocol specifies the WebSocket subprotocol name.
const streamWebSocketSubprotocol = constants.AppName + "_stream"

type ConsumerStarter interface {
	ConsumeStream(account *entity.Account, streamName string, handler func(msg *commandv1.ReplyMessage)) (StreamConsumer, error)
}

// StreamWebSocketHandler handles stream related WebSocket requests.
type StreamWebSocketHandler struct {
	accReader         AccountReader
	consumerStarter   ConsumerStarter
	heartbeatInterval time.Duration
	logger            log.Logger
	numConns          atomic.Int64
}

// NewStreamWebSocketHandler creates a new StreamWebSocketHandler.
func NewStreamWebSocketHandler(accounts AccountReader, consumerStarter ConsumerStarter) *StreamWebSocketHandler {
	return &StreamWebSocketHandler{
		accReader:         accounts,
		consumerStarter:   consumerStarter,
		heartbeatInterval: consumerHeartbeatInterval,
		logger:            log.NewLogger(),
		numConns:          atomic.Int64{},
	}
}

func (s *StreamWebSocketHandler) Register(g *echo.Group) {
	account := g.Group(fmt.Sprintf("%s/namespaces/:%s/accounts/:%s", entity.VersionPath, entity.PathParamNamespaceID, entity.PathParamAccountID))
	account.GET("/streams/consume", s.HandleConsume)
}

func (s *StreamWebSocketHandler) NumConnections() int64 {
	return s.numConns.Load()
}

func (s *StreamWebSocketHandler) HandleConsume(c echo.Context) (err error) {
	ctx := c.Request().Context()
	logger := s.logger.With("websocket.id", uuid.New().String())
	wg := new(sync.WaitGroup)

	conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		Subprotocols: []string{streamWebSocketSubprotocol},
	})
	if err != nil {
		return err
	}
	s.numConns.Add(1)

	var errMsg = "internal server error"
	var errDetails []string

	defer func() {
		code, reason := getWebSocketCloseCodeAndReason(err)
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
		closeWebSocketConn(conn, logger, code, reason)
		wg.Wait()
		s.numConns.Add(-1)
	}()

	var startReq StartStreamConsumerRequest
	if err = wsjson.Read(ctx, conn, &startReq); err != nil {
		return err
	}

	if err = startReq.Validate(); err != nil {
		errMsg, errDetails = "invalid start consumer message", valgoutil.GetDetails(err.(*valgo.Error))
		return err
	}

	accID := entity.MustParseID[entity.AccountID](startReq.AccountID)
	logger = logger.With(log.KeyAccountID, accID)

	acc, err := s.accReader.ReadAccount(ctx, accID)
	if err != nil {
		errMsg = "account not found"
		return err
	}

	if err = entity.VerifyEntityNamespace(c, acc); err != nil {
		errMsg = "unauthorized"
		return err
	}

	errCh := make(chan error, 2)

	consumer, err := s.consumerStarter.ConsumeStream(acc, startReq.Stream, func(msg *commandv1.ReplyMessage) {
		if werr := wsjson.Write(ctx, conn, server.Response[[]byte]{
			Data: msg.Data,
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

	for {
		select {
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
