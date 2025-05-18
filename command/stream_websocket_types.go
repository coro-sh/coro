package command

import (
	"github.com/cohesivestack/valgo"

	"github.com/coro-sh/coro/entity"
)

type StartStreamConsumerRequest struct {
	AccountID string `json:"account_id"`
	Stream    string `json:"stream"`
}

func (r StartStreamConsumerRequest) Validate() error {
	return valgo.Is(
		entity.IDValidator[entity.AccountID](r.AccountID, entity.PathParamAccountID),
		valgo.String(r.Stream, "stream").Not().Blank(),
	).Error()
}

type StreamConsumerMessage struct {
}
