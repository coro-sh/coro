package tx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"modernc.org/sqlite"
	lib "modernc.org/sqlite/lib"

	"github.com/coro-sh/coro/errtag"
)

// SQLiteTxer starts SQLite transactions (typically *sql.DB).
type SQLiteTxer interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	Conn(ctx context.Context) (*sql.Conn, error)
}

type SQLiteRepositoryTxerConfig[R any] struct {
	// Timeout is the maximum duration allowed for the entire transaction. Must
	// be a positive duration up to 10 seconds otherwise DefaultTimeout
	// is used.
	//
	// Timeout is applied by:
	//   - Running the transaction under a context deadline.
	//   - Setting PRAGMA busy_timeout to the same duration (lock wait cap).
	Timeout time.Duration

	// WithTxFunc returns a tx-bound copy of the repo using the provided
	// transaction. If the SQLiteRepositoryTxer already represents an in-flight
	// transaction, the original repo is returned unchanged (ambient tx reuse).
	//
	// The function must:
	//   - Clone the repo value passed in.
	//   - Bind the clone to the provided transaction (e.g. sqlc.Queries.WithTx).
	//   - Set the provided *SQLiteRepositoryTxer on the clone so nested calls
	//     reuse the ambient transaction.
	//   - Return the clone.
	//
	// NOTE: WithTxFunc receives a copied SQLiteRepositoryTxer whose transaction
	// is set for the lifetime of the new repository instance.
	WithTxFunc func(repo R, txer *SQLiteRepositoryTxer[R], tx *sql.Tx) R
}

// SQLiteRepositoryTxer adds transactional behavior to any SQLite repository
// type R, where the underlying database driver uses modernc.org/sqlite.
//
// It is typically stored on the repository (often as a pointer) and used to:
//  1. Start a transaction and return a repository *copy* bound to that tx.
//  2. Run a function inside a transaction with automatic commit/rollback.
//  3. Reuse an existing transaction for nested calls (no save points).
//
// Concurrency & Lifetime
//
//   - The root (non-transactional) repository that holds a *SQLiteRepositoryTxer
//     may be shared across goroutines.
//   - A repository instance with an active tx produced by WithTx or BeginTxFunc
//     must not be used concurrently by multiple goroutines. It is bound to a
//     single sql.Tx (single connection), which is unsafe for concurrent use.
//   - Treat tx-bound repository instances as short-lived values scoped to a
//     single unit of work. Avoid storing them in long-lived shared structs.
type SQLiteRepositoryTxer[R any] struct {
	Config SQLiteRepositoryTxerConfig[R]

	// txer is the database connection responsible for beginning new transactions
	// (usually *sql.DB).
	txer SQLiteTxer //

	// txn is set only on an SQLiteRepositoryTxer copy when a transaction is
	// in-flight. If non-nil, calls to BeginTxFunc/WithTx reuse the existing
	// transaction instead of starting a new one (no save points).
	txn Tx
}

// NewSQLiteRepositoryTxer creates a SQLite txer with sane defaults.
func NewSQLiteRepositoryTxer[R any](db SQLiteTxer, cfg SQLiteRepositoryTxerConfig[R]) *SQLiteRepositoryTxer[R] {
	if cfg.Timeout == 0 || cfg.Timeout > 10*time.Second {
		cfg.Timeout = DefaultTimeout
	}
	return &SQLiteRepositoryTxer[R]{Config: cfg, txer: db}
}

// WithTx returns a tx-bound copy of repo using the provided transaction.
// If this SQLiteRepositoryTxer already represents an in-flight transaction
// (txn != nil), the original repo is returned unchanged (ambient tx reuse).
//
// Panics if tx is not an *SQLTxWrapper.
func (r *SQLiteRepositoryTxer[R]) WithTx(repo R, txn Tx) R {
	if r.txn != nil {
		return repo
	}
	sqlw, ok := txn.(*SQLTxWrapper)
	if !ok {
		panic("tx.SQLiteRepositoryTxer.WithTx: expected *tx.SQLTxWrapper")
	}
	cpy := *r
	cpy.txn = sqlw
	return r.Config.WithTxFunc(repo, &cpy, sqlw.GetSQLTx())
}

// BeginTxFunc starts a new transaction (unless an ambient one is already in
// progress), clones and binds a repository to that transaction, and invokes fn.
// On success, the transaction is committed; on error, it is rolled back. If an
// ambient transaction exists (txn != nil), it is reused and fn is called directly.
//
// Nested behavior:
//   - Nested calls reuse the ambient transaction. Save points are not created.
//
// Panic semantics:
//   - If fn panics, the helper attempts to roll back the transaction and then
//     re-panics. If rollback itself fails, the panic is annotated accordingly.
func (r *SQLiteRepositoryTxer[R]) BeginTxFunc(
	ctx context.Context,
	repo R,
	fn func(ctx context.Context, tx Tx, repo R) error,
) error {
	if r.txn != nil {
		return fn(ctx, r.txn, repo)
	}

	if r.Config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Config.Timeout)
		defer cancel()
	}

	sqlTx, err := r.txer.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return TagSQLiteTimeoutErr(err)
	}

	if r.Config.Timeout > 0 {
		ms := int64(r.Config.Timeout / time.Millisecond)
		if _, err = sqlTx.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", ms)); err != nil {
			return TagSQLiteTimeoutErr(err)
		}
	}

	w := NewSQLTxWrapper(sqlTx)
	repoTx := r.WithTx(repo, w)

	if err := Do(ctx, w, func(ctx context.Context) error {
		return fn(ctx, w, repoTx)
	}); err != nil {
		return TagSQLiteTimeoutErr(err)
	}
	return nil
}

// InTx reports whether this txer is currently inside a transaction.
func (r *SQLiteRepositoryTxer[R]) InTx() bool { return r.txn != nil }

func TagSQLiteTimeoutErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errtag.Tag[ErrTagTransactionTimeout](err)
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case lib.SQLITE_BUSY, lib.SQLITE_LOCKED:
			return errtag.Tag[ErrTagTransactionTimeout](err)
		}
	}
	return err
}
