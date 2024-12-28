package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/coro-sh/coro/postgres/sqlc"
	"github.com/coro-sh/coro/tx"
)

type PGXTxer interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

var _ tx.Txer = (*Txer)(nil)

type Txer struct {
	pgxTxer PGXTxer
}

func NewTxer(pgxTxer PGXTxer) *Txer {
	return &Txer{
		pgxTxer: pgxTxer,
	}
}

func (t *Txer) BeginTx(ctx context.Context) (tx.Tx, error) {
	return t.pgxTxer.BeginTx(ctx, pgx.TxOptions{})
}

func initWithTx[T any](txn tx.Tx, initFn func(dbtx sqlc.DBTX) T) (T, error) {
	var t T

	pgxTx, ok := txn.(pgx.Tx)
	if !ok {
		return t, errors.New("tx does not implement pgx.Tx")
	}

	return initFn(pgxTx), nil
}
