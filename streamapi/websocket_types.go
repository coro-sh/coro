package streamapi

import (
	"github.com/cohesivestack/valgo"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/entityapi"
)

type StartStreamConsumerRequest struct {
	AccountID     string `param:"account_id" json:"-"`
	StreamName    string `param:"stream_name" json:"-"`
	StartSequence uint64 `query:"start_sequence"`
}

func (r StartStreamConsumerRequest) Validate() error {
	return valgo.In("params", valgo.Is(
		entityapi.IDValidator[entity.AccountID](r.AccountID, entityapi.PathParamAccountID),
		valgo.String(r.StreamName, "stream_name").Not().Blank(),
	)).ToError()
}

type StreamConsumerMessage struct {
}
