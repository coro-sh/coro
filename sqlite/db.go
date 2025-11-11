package sqlite

import (
	"github.com/coro-sh/coro/sqlite/sqlc"
	"github.com/coro-sh/coro/tx"
)

type DB interface {
	sqlc.DBTX
	tx.SQLiteTxer
}
