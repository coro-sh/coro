package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/coro-sh/coro/sqlite/sqlc"
	"github.com/coro-sh/coro/tx"
)

type SQLTxer interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

var _ tx.Txer = (*Txer)(nil)

type Txer struct {
	sqlTxer SQLTxer
}

func NewTxer(sqlTxer SQLTxer) *Txer {
	return &Txer{
		sqlTxer: sqlTxer,
	}
}

func (t *Txer) BeginTx(ctx context.Context) (tx.Tx, error) {
	sqlTx, err := t.sqlTxer.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	return &txWrapper{
		base: sqlTx,
	}, nil
}

func initWithTx[T any](txn tx.Tx, initFn func(dbtx sqlc.DBTX) T) (T, error) {
	var t T

	w, ok := txn.(*txWrapper)
	if !ok {
		return t, errors.New("tx is not a sqlite txWrapper")
	}

	return initFn(w.base), nil
}

var _ tx.Tx = (*txWrapper)(nil)

type txWrapper struct {
	base *sql.Tx
}

func (t *txWrapper) Commit(_ context.Context) error {
	return t.base.Commit()
}

func (t *txWrapper) Rollback(_ context.Context) error {
	return t.base.Rollback()
}
