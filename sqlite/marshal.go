package sqlite

import (
	"time"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/ref"
	"github.com/coro-sh/coro/sqlite/sqlc"
)

func unmarshalNamespace(in *sqlc.Namespace) *entity.Namespace {
	return &entity.Namespace{
		ID:    entity.MustParseID[entity.NamespaceID](in.ID),
		Name:  in.Name,
		Owner: in.Owner,
	}
}

func unmarshalOperator(in *sqlc.Operator) entity.OperatorData {
	return entity.OperatorData{
		OperatorIdentity: entity.OperatorIdentity{
			ID:          entity.MustParseID[entity.OperatorID](in.ID),
			NamespaceID: entity.MustParseID[entity.NamespaceID](in.NamespaceID),
			JWT:         in.Jwt,
		},
		Name:      in.Name,
		PublicKey: in.PublicKey,
	}
}

func unmarshalAccount(in *sqlc.Account) entity.AccountData {
	out := entity.AccountData{
		AccountIdentity: entity.AccountIdentity{
			ID:          entity.MustParseID[entity.AccountID](in.ID),
			NamespaceID: entity.MustParseID[entity.NamespaceID](in.NamespaceID),
			OperatorID:  entity.MustParseID[entity.OperatorID](in.OperatorID),
			JWT:         in.Jwt,
		},
		Name:      in.Name,
		PublicKey: in.PublicKey,
	}
	if in.UserJwtDuration != nil {
		out.UserJWTDuration = ref.Ptr(time.Second * time.Duration(*in.UserJwtDuration))
	}
	return out
}

func unmarshalUser(in *sqlc.User) entity.UserData {
	out := entity.UserData{
		UserIdentity: entity.UserIdentity{
			ID:          entity.MustParseID[entity.UserID](in.ID),
			NamespaceID: entity.MustParseID[entity.NamespaceID](in.NamespaceID),
			OperatorID:  entity.MustParseID[entity.OperatorID](in.OperatorID),
			AccountID:   entity.MustParseID[entity.AccountID](in.AccountID),
		},
		Name: in.Name,
		JWT:  in.Jwt,
	}
	if in.JwtDuration != nil {
		out.JWTDuration = ref.Ptr(time.Second * time.Duration(*in.JwtDuration))
	}
	return out
}

func unmarshalUserJWTIssuances(in *sqlc.UserJwtIssuance) entity.UserJWTIssuance {
	return entity.UserJWTIssuance{
		IssueTime:  in.IssueTime,
		ExpireTime: in.ExpireTime,
	}
}

func unmarshalNkey(in *sqlc.Nkey) entity.NkeyData {
	return entity.NkeyData{
		ID:         in.ID,
		Type:       entity.ParseTypeFromString(in.Type),
		Seed:       in.Seed,
		SigningKey: false,
	}
}

func unmarshalSigningKey(in *sqlc.SigningKey) entity.NkeyData {
	return entity.NkeyData{
		ID:         in.ID,
		Type:       entity.ParseTypeFromString(in.Type),
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
