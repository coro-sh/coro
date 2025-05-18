package nscmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/log"
	"github.com/coro-sh/coro/server"
	"github.com/coro-sh/coro/tkn"
)

const (
	pathParamNamespaceID = "namespace_id"
	pathParamOperatorID  = "operator_id"
)

type OperatorReader interface {
	// ReadOperator reads an Operator by its ID.
	ReadOperator(ctx context.Context, id entity.OperatorID) (*entity.Operator, error)
}

type Pinger interface {
	// Ping checks if Operator's have an open proxy connection a NATS server.
	Ping(ctx context.Context, operatorID entity.OperatorID) (entity.OperatorNATSStatus, error)
}

// ProxyHTTPHandler handles proxy related HTTP requests.
type ProxyHTTPHandler struct {
	iss            *tkn.OperatorIssuer
	operatorLoader OperatorReader
	pinger         Pinger
}

// NewProxyHTTPHandler creates a new ProxyHTTPHandler.
func NewProxyHTTPHandler(iss *tkn.OperatorIssuer, operators OperatorReader, pinger Pinger) *ProxyHTTPHandler {
	return &ProxyHTTPHandler{
		iss:            iss,
		operatorLoader: operators,
		pinger:         pinger,
	}
}

func (h *ProxyHTTPHandler) Register(g *echo.Group) {
	v1 := g.Group(versionPath)
	v1.POST(fmt.Sprintf("/namespaces/:%s/operators/:%s/proxy/token", pathParamNamespaceID, pathParamOperatorID), h.GenerateProxyToken)
	v1.GET(fmt.Sprintf("/namespaces/:%s/operators/:%s/proxy/status", pathParamNamespaceID, pathParamOperatorID), h.GetProxyStatus)
}

// GenerateProxyToken handles POST requests to generate a Proxy token for an
// Operator, which is used to authorize a NATS proxy WebSocket connection.
// Only one token can be active at a given time. Generating a new a token wil
// overwrite and invalidate any pre-existing token.
func (h *ProxyHTTPHandler) GenerateProxyToken(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GenerateProxyTokenRequest](c)
	if err != nil {
		return err
	}
	opID := entity.MustParseID[entity.OperatorID](req.OperatorID)
	c.Set(log.KeyOperatorID, opID)

	op, err := h.operatorLoader.ReadOperator(ctx, opID)
	if err != nil {
		return err
	}

	if err = entity.VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	token, err := h.iss.Generate(ctx, opID)
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, &GenerateProxyTokenResponse{
		Token: token,
	})
}

// GetProxyStatus handles GET requests to check if an Operator has an active
// Proxy connection open between the Broker WebSocket server and the Operator's
// NATS server.
func (h *ProxyHTTPHandler) GetProxyStatus(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetProxyStatusRequest](c)
	if err != nil {
		return err
	}
	opID := entity.MustParseID[entity.OperatorID](req.OperatorID)
	c.Set(log.KeyOperatorID, opID)

	op, err := h.operatorLoader.ReadOperator(ctx, opID)
	if err != nil {
		return err
	}

	if err = entity.VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	opStatus, err := h.pinger.Ping(pctx, op.ID)
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, opStatus)
}
