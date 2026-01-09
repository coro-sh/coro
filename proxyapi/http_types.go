package proxyapi

import (
	"github.com/cohesivestack/valgo"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/entityapi"
)

type GenerateProxyTokenRequest struct {
	OperatorID string `param:"operator_id" json:"-"`
}

func (r GenerateProxyTokenRequest) Validate() error {
	return valgo.In("params",
		valgo.Is(entityapi.IDValidator[entity.OperatorID](r.OperatorID, entityapi.PathParamOperatorID)),
	).ToError()
}

type GenerateProxyTokenResponse struct {
	Token string `json:"token"`
}

type GetProxyStatusRequest struct {
	OperatorID string `param:"operator_id" json:"-"`
}

func (r GetProxyStatusRequest) Validate() error {
	return valgo.In("params",
		valgo.Is(entityapi.IDValidator[entity.OperatorID](r.OperatorID, entityapi.PathParamOperatorID)),
	).ToError()
}

type GetProxyStatusResponse struct {
	Connected bool `json:"connected"`
}
