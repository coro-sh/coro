package command

import (
	"github.com/cohesivestack/valgo"

	"github.com/coro-sh/coro/entity"
)

type StartStreamConsumerRequest struct {
	AccountID     string `param:"account_id" json:"-"`
	StreamName    string `param:"stream_name" json:"-"`
	StartSequence uint64 `query:"start_sequence"`
}

func (r StartStreamConsumerRequest) Validate() error {
	return valgo.In("params", valgo.Is(
		entity.IDValidator[entity.AccountID](r.AccountID, entity.PathParamAccountID),
		valgo.String(r.StreamName, "stream_name").Not().Blank(),
	)).Error()
}

type StreamConsumerMessage struct {
}
