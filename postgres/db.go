package postgres

import (
	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/postgres/sqlc"
	"github.com/coro-sh/coro/tx"
)

const AppDBName = constants.AppName

type DB interface {
	sqlc.DBTX
	tx.PGXTxer
}
