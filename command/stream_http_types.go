package command

import (
	"github.com/cohesivestack/valgo"

	"github.com/coro-sh/coro/entity"
)

type ListStreamsRequest struct {
	AccountID string `param:"account_id" json:"-"`
}

func (r ListStreamsRequest) Validate() error {
	return valgo.In("params",
		valgo.Is(entity.IDValidator[entity.AccountID](r.AccountID, entity.PathParamAccountID)),
	).Error()
}

type StreamResponse struct {
	Name          string   `json:"name"`
	Subjects      []string `json:"subjects"`
	MessageCount  uint64   `json:"message_count"`
	ConsumerCount int      `json:"consumer_count"`
	CreateTime    int64    `json:"create_time"` // unix milli
}
