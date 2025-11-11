package tx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/coro-sh/coro/errtag"
)

// PGXTxer is implemented by types that can begin a pgx-backed transaction.
// In pgx/v5 both pgxpool.Pool and pgx.Conn expose BeginTx methods that satisfy
// this interface.
type PGXTxer interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type PGXRepositoryTxerConfig[R any] struct {
	// Timeout is the maximum duration allowed for the entire transaction. Must
	// be a positive duration up to 10 seconds otherwise DefaultTimeout
	// is used.
	Timeout time.Duration

	// WithTxFunc returns a tx-bound copy of the repo using the provided
	// transaction. If the PGXRepositoryTxer already represents an in-flight
	// transaction, the original repo is returned unchanged (ambient tx reuse).
	//
	// The function must:
	//   - Clone the repo value passed in.
	//   - Bind the clone to the provided transaction (e.g. sqlc.Queries.WithTx).
	//   - Set the provided *PGXRepositoryTxer on the clone so nested calls reuse
	//     the ambient transaction.
	//   - Return the clone.
	//
	// NOTE: WithTxFunc receives a copied PGXRepositoryTxer whose transaction is
	// set for the lifetime of the new repository instance.
	WithTxFunc func(repo R, txer *PGXRepositoryTxer[R], tx pgx.Tx) R
}

// PGXRepositoryTxer adds transactional behavior to any repository type R.
// It is typically stored on the repository (often as a pointer) and used to:
//  1. Start a transaction and return a repository *copy* bound to that tx.
//  2. Run a function inside a transaction with automatic commit/rollback.
//  3. Reuse an existing transaction for nested calls (no save points).
//
// Concurrency & Lifetime
//
//   - The root (non-transactional) repository that holds a *PGXRepositoryTxer
//     may be shared across goroutines as long as its underlying DB handle is a pool.
//   - A repository instance with an active tx produced by WithTx or BeginTxFunc
//     must not be used concurrently by multiple goroutines. It is bound to a
//     single pgx.Tx (single connection), which is unsafe for concurrent use.
//   - Treat tx-bound repository instances as short-lived values scoped to a
//     single unit of work. Avoid storing them in long-lived shared structs.
type PGXRepositoryTxer[R any] struct {
	Config PGXRepositoryTxerConfig[R]

	// txer is the Postgres connection responsible for beginning new transactions.
	txer PGXTxer

	// txn is set only on a PGXRepositoryTxer copy when a transaction is in-flight.
	// If non-nil, calls to BeginTxFunc/WithTx reuse the existing transaction
	// instead of starting a new one (no save points).
	txn Tx
}

// NewPGXRepositoryTxer constructs a PGXRepositoryTxer for a concrete repository
// type R. The withTx function is repository-specific and is responsible for
// cloning and binding the repo to the provided pgx.Tx (see withTx doc above).
func NewPGXRepositoryTxer[R any](txer PGXTxer, cfg PGXRepositoryTxerConfig[R]) *PGXRepositoryTxer[R] {
	if cfg.Timeout == 0 || cfg.Timeout > 10*time.Second {
		cfg.Timeout = DefaultTimeout
	}
	return &PGXRepositoryTxer[R]{
		Config: cfg,
		txer:   txer,
	}
}

// WithTx returns a tx-bound copy of repo using the provided transaction.
// If this PGXRepositoryTxer already represents an in-flight transaction
// (txn != nil), the original repo is returned unchanged (ambient tx reuse).
//
// Panics if tx does not implement pgx.Tx.
func (r *PGXRepositoryTxer[R]) WithTx(repo R, tx Tx) R {
	if r.txn != nil {
		return repo // tx already in progress
	}

	pgxTx, ok := tx.(pgx.Tx)
	if !ok {
		panic("tx.PGXRepositoryTxer.WithTx: expected pgx.Tx")
	}

	cpy := *r
	cpy.txn = pgxTx
	return r.Config.WithTxFunc(repo, &cpy, pgxTx)
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
func (r *PGXRepositoryTxer[R]) BeginTxFunc(ctx context.Context, repo R, fn func(ctx context.Context, tx Tx, repo R) error) error {
	if r.txn != nil {
		return fn(ctx, r.txn, repo)
	}

	txn, err := r.txer.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	timeoutMS := r.Config.Timeout.Milliseconds()
	if _, err := txn.Exec(ctx, fmt.Sprintf("SET LOCAL transaction_timeout = '%dms'", timeoutMS)); err != nil {
		return err
	}
	if _, err := txn.Exec(ctx, fmt.Sprintf("SET LOCAL idle_in_transaction_session_timeout = '%dms'", timeoutMS)); err != nil {
		return err
	}

	if err = Do(ctx, txn, func(ctx context.Context) error {
		return fn(ctx, txn, r.WithTx(repo, txn))
	}); err != nil {
		return TagPGXTimeoutErr(err)
	}

	return nil
}

// InTx reports whether this txer is currently inside a transaction.
func (r *PGXRepositoryTxer[R]) InTx() bool {
	return r.txn != nil
}

func TagPGXTimeoutErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == pgerrcode.IdleInTransactionSessionTimeout || pgErr.Code == "25P04") {
		err = errtag.Tag[ErrTagTransactionTimeout](err)
	}
	return err
}
