package nscmd

import (
	"github.com/cohesivestack/valgo"

	"github.com/coro-sh/coro/entity"
)

type GenerateProxyTokenRequest struct {
	OperatorID string `param:"operator_id" json:"-"`
}

func (r GenerateProxyTokenRequest) Validate() error {
	return valgo.In("params",
		valgo.Is(entity.IDValidator[entity.OperatorID](r.OperatorID, pathParamOperatorID)),
	).Error()
}

type GenerateProxyTokenResponse struct {
	Token string `json:"token"`
}

type GetProxyStatusRequest struct {
	OperatorID string `param:"operator_id" json:"-"`
}

func (r GetProxyStatusRequest) Validate() error {
	return valgo.In("params",
		valgo.Is(entity.IDValidator[entity.OperatorID](r.OperatorID, pathParamOperatorID)),
	).Error()
}

type GetProxyStatusResponse struct {
	Connected bool `json:"connected"`
}
