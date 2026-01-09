package command

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/joshjon/kit/errtag"
	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/logkey"
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
)

const (
	PathParamStreamName     = "stream_name"
	PathParamStreamSequence = "stream_sequence"
)

type AccountReader interface {
	// ReadAccount reads an Account by its ID.
	ReadAccount(ctx context.Context, id entity.AccountID) (*entity.Account, error)
}

type StreamReader interface {
	ListStreams(ctx context.Context, account *entity.Account) ([]*jetstream.StreamInfo, error)
	GetStream(ctx context.Context, account *entity.Account, streamName string) (*jetstream.StreamInfo, error)
	FetchStreamMessages(ctx context.Context, account *entity.Account, streamName string, startSeq uint64, batchSize uint32) (*commandv1.StreamMessageBatch, error)
	GetStreamMessageContent(ctx context.Context, account *entity.Account, streamName string, seq uint64) (*commandv1.StreamMessageContent, error)
}

// StreamHTTPHandler handles stream related HTTP requests.
type StreamHTTPHandler struct {
	accReader    AccountReader
	streamReader StreamReader
}

// NewStreamHTTPHandler creates a new StreamHTTPHandler.
func NewStreamHTTPHandler(accounts AccountReader, streams StreamReader) *StreamHTTPHandler {
	return &StreamHTTPHandler{
		accReader:    accounts,
		streamReader: streams,
	}
}

func (h *StreamHTTPHandler) Register(g *echo.Group) {
	account := g.Group(fmt.Sprintf("/namespaces/:%s/accounts/:%s", entity.PathParamNamespaceID, entity.PathParamAccountID))
	account.GET("/streams", h.ListStreams)
	account.GET(fmt.Sprintf("/streams/:%s", PathParamStreamName), h.GetStream)
	account.GET(fmt.Sprintf("/streams/:%s/messages", PathParamStreamName), h.FetchStreamMessages)
	account.GET(fmt.Sprintf("/streams/:%s/messages/:%s", PathParamStreamName, PathParamStreamSequence), h.GetStreamMessageContent)
}

// ListStreams handles GET requests to list all streams for an Account.
func (h *StreamHTTPHandler) ListStreams(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[ListStreamsRequest](c)
	if err != nil {
		return err
	}
	accID := entity.MustParseID[entity.AccountID](req.AccountID)
	c.Set(logkey.AccountID, accID)

	acc, err := h.accReader.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}

	if err = entity.VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	streams, err := h.streamReader.ListStreams(ctx, acc)
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
			CreateTime:    stream.Created.Unix(),
		}
	}

	return server.SetResponseList(c, http.StatusOK, streamResps, "")
}

// GetStream handles GET requests to get a stream for an Account.
func (h *StreamHTTPHandler) GetStream(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetStreamRequest](c)
	if err != nil {
		return err
	}
	accID := entity.MustParseID[entity.AccountID](req.AccountID)
	c.Set(logkey.AccountID, accID)
	c.Set(logkey.StreamName, req.StreamName)

	acc, err := h.accReader.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}

	if err = entity.VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	stream, err := h.streamReader.GetStream(ctx, acc, req.StreamName)
	if err != nil {
		if strings.HasSuffix(err.Error(), nats.ErrStreamNotFound.Error()) {
			return errtag.Tag[errtag.NotFound](err, errtag.WithMsg("Stream not found"))
		}
		return err
	}

	return server.SetResponse(c, http.StatusOK, StreamResponse{
		Name:          stream.Config.Name,
		Subjects:      stream.Config.Subjects,
		MessageCount:  stream.State.Msgs,
		ConsumerCount: stream.State.Consumers,
		CreateTime:    stream.Created.Unix(),
	})
}

// FetchStreamMessages handles GET requests to fetch a batch of messages that
// are currently available in a stream. Does not wait for new messages to
// arrive, even if batch size is not met.
func (h *StreamHTTPHandler) FetchStreamMessages(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[FetchStreamMessagesRequest](c)
	if err != nil {
		return err
	}
	accID := entity.MustParseID[entity.AccountID](req.AccountID)
	c.Set(logkey.AccountID, accID)
	c.Set(logkey.StreamName, req.StreamName)
	c.Set(logkey.ConsumerStartSequence, req.StartSequence)
	c.Set(logkey.ConsumerFetchBatchSize, req.BatchSize)

	acc, err := h.accReader.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}

	if err = entity.VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	batch, err := h.streamReader.FetchStreamMessages(ctx, acc, req.StreamName, req.StartSequence, req.BatchSize)
	if err != nil {
		return err
	}

	return server.SetResponseList(c, http.StatusOK, batch.Messages, "")
}

// GetStreamMessageContent handles GET requests to get the content of message
// that is currently available in a stream.
func (h *StreamHTTPHandler) GetStreamMessageContent(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetStreamMessageContentRequest](c)
	if err != nil {
		return err
	}
	accID := entity.MustParseID[entity.AccountID](req.AccountID)
	c.Set(logkey.AccountID, accID)

	acc, err := h.accReader.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}

	if err = entity.VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	msg, err := h.streamReader.GetStreamMessageContent(ctx, acc, req.StreamName, req.Sequence)
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, msg)
}
