package streamapi

import (
	"github.com/cohesivestack/valgo"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/entityapi"
)

type ListStreamsRequest struct {
	AccountID string `param:"account_id" json:"-"`
}

func (r ListStreamsRequest) Validate() error {
	return valgo.In("params", valgo.Is(
		entityapi.IDValidator[entity.AccountID](r.AccountID, entityapi.PathParamAccountID),
	)).ToError()
}

type GetStreamRequest struct {
	AccountID  string `param:"account_id" json:"-"`
	StreamName string `param:"stream_name" json:"-"`
}

func (r GetStreamRequest) Validate() error {
	return valgo.In("params", valgo.Is(
		entityapi.IDValidator[entity.AccountID](r.AccountID, entityapi.PathParamAccountID),
		valgo.String(r.StreamName, "stream_name").Not().Blank(),
	)).ToError()
}

type FetchStreamMessagesRequest struct {
	AccountID     string `param:"account_id" json:"-"`
	StreamName    string `param:"stream_name" json:"-"`
	StartSequence uint64 `query:"start_sequence"`
	BatchSize     uint32 `query:"batch_size"`
}

func (r FetchStreamMessagesRequest) Validate() error {
	v := valgo.New()
	v.In("params", valgo.Is(
		entityapi.IDValidator[entity.AccountID](r.AccountID, entityapi.PathParamAccountID),
		valgo.String(r.StreamName, "stream_name").Not().Blank(),
	))
	v.In("query_params", valgo.Is(
		valgo.Uint32(r.BatchSize, "batch_size").LessOrEqualTo(1000),
	))
	return v.ToError()
}

type GetStreamMessageContentRequest struct {
	AccountID  string `param:"account_id" json:"-"`
	StreamName string `param:"stream_name" json:"-"`
	Sequence   uint64 `param:"stream_sequence"`
}

func (r GetStreamMessageContentRequest) Validate() error {
	return valgo.In("params",
		valgo.Is(
			entityapi.IDValidator[entity.AccountID](r.AccountID, entityapi.PathParamAccountID),
			valgo.String(r.StreamName, "stream_name").Not().Blank(),
			valgo.Uint64(r.Sequence, "stream_sequence").GreaterOrEqualTo(1),
		),
	).Error()
}

type StreamResponse struct {
	Name          string   `json:"name"`
	Subjects      []string `json:"subjects"`
	MessageCount  uint64   `json:"message_count"`
	ConsumerCount int      `json:"consumer_count"`
	CreateTime    int64    `json:"create_time"` // unix milli
}
