package websocketutil

import (
	"context"
	"errors"
	"io"
	"net"

	"github.com/coder/websocket"
	"github.com/joshjon/kit/log"
)

func CloseConn(conn *websocket.Conn, code websocket.StatusCode, reason string, logger log.Logger) {
	if cerr := conn.Close(code, reason); cerr != nil && !errors.Is(cerr, net.ErrClosed) {
		logger.Error("failed to close websocket connection", "error", cerr, "websocket.close_code", code, "websocket.close_reason", reason)
		return
	}
	logger.Info("websocket connection closed", "websocket.close_code", code, "websocket.close_reason", reason)
}

func GetCloseErrCodeAndReason(err error) (websocket.StatusCode, string) {
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
