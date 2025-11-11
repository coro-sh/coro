package tkn

import (
	"context"
	"fmt"
	"strings"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
)

type OperatorTokenType string

const OperatorTokenTypeProxy OperatorTokenType = "pxy"

type OperatorIssuer struct {
	iss     *Issuer
	rw      OperatorTokenReadWriter
	tknType OperatorTokenType
}

func NewOperatorIssuer(rw OperatorTokenReadWriter, tknType OperatorTokenType) *OperatorIssuer {
	return &OperatorIssuer{
		iss:     NewIssuer(),
		rw:      rw,
		tknType: tknType,
	}
}

func (o *OperatorIssuer) Generate(ctx context.Context, operatorID entity.OperatorID) (string, error) {
	token, err := o.iss.Generate(WithPrefix(fmt.Sprintf("%s_%s", string(o.tknType), operatorID.Suffix())))
	if err != nil {
		return "", fmt.Errorf("generate proxy token: %w", err)
	}

	hashed, err := o.iss.Hash(token)
	if err != nil {
		return "", fmt.Errorf("hash proxy token: %w", err)
	}

	if err = o.rw.Write(ctx, o.tknType, operatorID, hashed); err != nil {
		return "", fmt.Errorf("write proxy token: %w", err)
	}

	return token, nil
}

func (o *OperatorIssuer) Verify(ctx context.Context, token string) (id entity.OperatorID, err error) {
	var zeroID entity.OperatorID
	tokenTypePrefix := string(o.tknType) + "_"

	if !strings.HasPrefix(token, tokenTypePrefix) {
		return zeroID, errtag.NewTagged[errtag.Unauthorized]("unrecognized token")
	}

	opIDSuffixLen := len(zeroID.Suffix())
	if len(token) < opIDSuffixLen {
		return zeroID, errtag.NewTagged[errtag.Unauthorized]("invalid token length")
	}

	opIDSuffix := strings.TrimPrefix(token, tokenTypePrefix)[:opIDSuffixLen]
	opIDStr := fmt.Sprintf("%s_%s", zeroID.Prefix(), opIDSuffix)
	operatorID, err := entity.ParseID[entity.OperatorID](opIDStr)
	if err != nil {
		return zeroID, errtag.Tag[errtag.Unauthorized](fmt.Errorf("parse operator id: %w", err))
	}

	hashed, err := o.rw.Read(ctx, o.tknType, operatorID)
	if err != nil {
		if errtag.HasTag[errtag.NotFound](err) {
			return zeroID, errtag.Tag[errtag.Unauthorized](err)
		}
		return zeroID, err
	}

	ok, err := o.iss.Verify(token, hashed)
	if err != nil {
		return zeroID, errtag.Tag[errtag.Unauthorized](err)
	}
	if !ok {
		return zeroID, errtag.NewTagged[errtag.Unauthorized]("token mismatch")
	}

	return operatorID, nil
}
