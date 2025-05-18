package command

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/log"
	"github.com/coro-sh/coro/server"
)

type AccountReader interface {
	// ReadAccount reads an Account by its ID.
	ReadAccount(ctx context.Context, id entity.AccountID) (*entity.Account, error)
}

type StreamLister interface {
	ListStreams(ctx context.Context, account *entity.Account) ([]*jetstream.StreamInfo, error)
}

// StreamHTTPHandler handles stream related HTTP requests.
type StreamHTTPHandler struct {
	accReader    AccountReader
	streamLister StreamLister
}

// NewStreamHTTPHandler creates a new StreamHTTPHandler.
func NewStreamHTTPHandler(accounts AccountReader, streams StreamLister) *StreamHTTPHandler {
	return &StreamHTTPHandler{
		accReader:    accounts,
		streamLister: streams,
	}
}

func (h *StreamHTTPHandler) Register(g *echo.Group) {
	account := g.Group(fmt.Sprintf("%s/namespaces/:%s/accounts/:%s", entity.VersionPath, entity.PathParamNamespaceID, entity.PathParamAccountID))
	account.GET("/streams", h.ListStreams)
}

func (h *StreamHTTPHandler) ListStreams(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[ListStreamsRequest](c)
	if err != nil {
		return err
	}
	accID := entity.MustParseID[entity.AccountID](req.AccountID)
	c.Set(log.KeyAccountID, accID)

	acc, err := h.accReader.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}

	if err = entity.VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	streams, err := h.streamLister.ListStreams(ctx, acc)
	if err != nil {
		return err
	}

	streamResps := make([]StreamResponse, len(streams))

	for i, stream := range streams {
		streamResps[i] = StreamResponse{
			Name:          stream.Config.Name,
			Subjects:      stream.Config.Subjects,
			MessageCount:  stream.State.Msgs,
			ConsumerCount: stream.State.Consumers,
			CreateTime:    stream.Created.UnixMilli(),
		}
	}

	return server.SetResponseList(c, http.StatusOK, streamResps, "")
}
