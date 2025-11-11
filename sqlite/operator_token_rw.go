package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/sqlite/sqlc"
	"github.com/coro-sh/coro/tkn"
	"github.com/coro-sh/coro/tx"
)

var _ tkn.OperatorTokenReadWriter = (*OperatorTokenReadWriter)(nil)

type OperatorTokenReadWriter struct {
	db   *sqlc.Queries
	txer *tx.SQLiteRepositoryTxer[tkn.OperatorTokenReadWriter]
}

func NewOperatorTokenReadWriter(db DB) *OperatorTokenReadWriter {
	return &OperatorTokenReadWriter{
		db: sqlc.New(db),
		txer: tx.NewSQLiteRepositoryTxer(db, tx.SQLiteRepositoryTxerConfig[tkn.OperatorTokenReadWriter]{
			Timeout: tx.DefaultTimeout,
			WithTxFunc: func(repo tkn.OperatorTokenReadWriter, txer *tx.SQLiteRepositoryTxer[tkn.OperatorTokenReadWriter], tx *sql.Tx) tkn.OperatorTokenReadWriter {
				cpy := *repo.(*OperatorTokenReadWriter)
				cpy.db = cpy.db.WithTx(tx)
				cpy.txer = txer
				return tkn.OperatorTokenReadWriter(&cpy)
			},
		}),
	}
}

func (o *OperatorTokenReadWriter) Read(ctx context.Context, tokenType tkn.OperatorTokenType, operatorID entity.OperatorID) (string, error) {
	token, err := o.db.ReadOperatorToken(ctx, sqlc.ReadOperatorTokenParams{
		OperatorID: operatorID.String(),
		Type:       string(tokenType),
	})
	if err != nil {
		return "", tagOpTknErr(err)
	}
	return token, nil
}

func (o *OperatorTokenReadWriter) Write(ctx context.Context, tokenType tkn.OperatorTokenType, operatorID entity.OperatorID, hashedToken string) error {
	err := o.db.UpsertOperatorToken(ctx, sqlc.UpsertOperatorTokenParams{
		OperatorID: operatorID.String(),
		Type:       string(tokenType),
		Token:      hashedToken,
	})
	return tagOpTknErr(err)
}

func (o *OperatorTokenReadWriter) WithTx(tx tx.Tx) tkn.OperatorTokenReadWriter {
	return o.txer.WithTx(o, tx)
}

func (o *OperatorTokenReadWriter) BeginTxFunc(ctx context.Context, fn func(ctx context.Context, tx tx.Tx, repo tkn.OperatorTokenReadWriter) error) error {
	return o.txer.BeginTxFunc(ctx, o, fn)
}

func tagOpTknErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return errtag.Tag[errtag.NotFound](err, errtag.WithMsg("Operator token not found"))
	}
	return tagErr(err)
}
