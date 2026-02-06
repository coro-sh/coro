package postgres

import (
	"github.com/joshjon/kit/id"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/postgres/sqlc"
)

func unmarshalNamespace(in *sqlc.Namespace) *entity.Namespace {
	return &entity.Namespace{
		ID:    id.MustParse[entity.NamespaceID](in.ID),
		Name:  in.Name,
		Owner: in.Owner,
	}
}

func unmarshalOperator(in *sqlc.Operator) entity.OperatorData {
	return entity.OperatorData{
		OperatorIdentity: entity.OperatorIdentity{
			ID:          id.MustParse[entity.OperatorID](in.ID),
			NamespaceID: id.MustParse[entity.NamespaceID](in.NamespaceID),
			JWT:         in.Jwt,
		},
		Name:            in.Name,
		PublicKey:       in.PublicKey,
		LastConnectTime: in.LastConnectTime,
	}
}

func unmarshalAccount(in *sqlc.Account) entity.AccountData {
	return entity.AccountData{
		AccountIdentity: entity.AccountIdentity{
			ID:          id.MustParse[entity.AccountID](in.ID),
			NamespaceID: id.MustParse[entity.NamespaceID](in.NamespaceID),
			OperatorID:  id.MustParse[entity.OperatorID](in.OperatorID),
			JWT:         in.Jwt,
		},
		Name:            in.Name,
		PublicKey:       in.PublicKey,
		UserJWTDuration: in.UserJwtDuration,
	}
}

func unmarshalUser(in *sqlc.User) entity.UserData {
	return entity.UserData{
		UserIdentity: entity.UserIdentity{
			ID:          id.MustParse[entity.UserID](in.ID),
			NamespaceID: id.MustParse[entity.NamespaceID](in.NamespaceID),
			OperatorID:  id.MustParse[entity.OperatorID](in.OperatorID),
			AccountID:   id.MustParse[entity.AccountID](in.AccountID),
		},
		Name:        in.Name,
		JWT:         in.Jwt,
		JWTDuration: in.JwtDuration,
	}
}

func unmarshalUserJWTIssuances(in *sqlc.UserJwtIssuance) entity.UserJWTIssuance {
	return entity.UserJWTIssuance{
		IssueTime:  in.IssueTime,
		ExpireTime: in.ExpireTime,
	}
}

func marshalNkeyType(in entity.Type) sqlc.NkeyType {
	switch in {
	case entity.TypeOperator:
		return sqlc.NkeyTypeOperator
	case entity.TypeAccount:
		return sqlc.NkeyTypeAccount
	case entity.TypeUser:
		return sqlc.NkeyTypeUser
	case entity.TypeUnspecified:
		fallthrough
	default:
		return ""
	}
}

func unmarshalNkeyType(in sqlc.NkeyType) entity.Type {
	switch in {
	case sqlc.NkeyTypeOperator:
		return entity.TypeOperator
	case sqlc.NkeyTypeAccount:
		return entity.TypeAccount
	case sqlc.NkeyTypeUser:
		return entity.TypeUser
	default:
		return entity.TypeUnspecified
	}
}

func unmarshalNkey(in *sqlc.Nkey) entity.NkeyData {
	return entity.NkeyData{
		ID:         in.ID,
		Type:       unmarshalNkeyType(in.Type),
		Seed:       in.Seed,
		SigningKey: false,
	}
}

func unmarshalSigningKey(in *sqlc.SigningKey) entity.NkeyData {
	return entity.NkeyData{
		ID:         in.ID,
		Type:       unmarshalNkeyType(in.Type),
		Seed:       in.Seed,
		SigningKey: true,
	}
}

func unmarshalList[T any, U any](in []T, unmarshaler func(in T) U) []U {
	out := make([]U, len(in))
	for i := range in {
		out[i] = unmarshaler(in[i])
	}
	return out
}
