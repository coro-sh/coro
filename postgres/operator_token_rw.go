package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/postgres/sqlc"
	"github.com/coro-sh/coro/tkn"
)

var _ tkn.OperatorTokenReadWriter = (*OperatorTokenReadWriter)(nil)

type OperatorTokenReadWriter struct {
	db *sqlc.Queries
}

func NewOperatorTokenReadWriter(dbtx sqlc.DBTX) *OperatorTokenReadWriter {
	return &OperatorTokenReadWriter{
		db: sqlc.New(dbtx),
	}
}

func (o *OperatorTokenReadWriter) Read(ctx context.Context, tokenType tkn.OperatorTokenType, operatorID entity.OperatorID) (string, error) {
	token, err := o.db.ReadOperatorToken(ctx, sqlc.ReadOperatorTokenParams{
		OperatorID: operatorID.String(),
		Type:       string(tokenType),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errtag.Tag[errtag.NotFound](err, errtag.WithMsg("Operator token not found"))
		}
		return "", err
	}
	return token, nil
}

func (o *OperatorTokenReadWriter) Write(ctx context.Context, tokenType tkn.OperatorTokenType, operatorID entity.OperatorID, hashedToken string) error {
	return o.db.UpsertOperatorToken(ctx, sqlc.UpsertOperatorTokenParams{
		OperatorID: operatorID.String(),
		Type:       string(tokenType),
		Token:      hashedToken,
	})
}
