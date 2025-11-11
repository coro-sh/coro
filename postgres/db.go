package postgres

import (
	"github.com/coro-sh/coro/postgres/sqlc"
	"github.com/coro-sh/coro/tx"
)

type DB interface {
	sqlc.DBTX
	tx.PGXTxer
}
