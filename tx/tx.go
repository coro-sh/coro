package tx

import (
	"context"
	"fmt"
	"time"
)

const DefaultTimeout = 10 * time.Second

type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Repository is a generic interface implemented by repository types that support
// transactional operations. It defines the minimum set of methods required to
// integrate with tx helpers such as PGXRepositoryTxer and SQLRepositoryTxer.
//
// The type parameter R represents the concrete repository type, allowing
// WithTx and BeginTxFunc to return or receive type-safe repository instances.
//
// Typical usage:
//
//	repo.BeginTxFunc(ctx, func(ctx context.Context, tx tx.Tx, repo MyRepository) error {
//	    // Perform one or more operations using the tx-bound repository.
//	    if err := repo.CreateEntity(ctx, entity); err != nil {
//	        return err
//	    }
//	    return nil
//	})
//
// Concurrency & Lifetime
//
//   - The root (non-transactional) repository may be shared safely across
//     goroutines if its underlying database handle supports concurrency
//     (e.g. *pgxpool.Pool or *sql.DB).
//   - A repository instance returned by WithTx or created within BeginTxFunc
//     is bound to a single transaction (and thus a single connection) and must
//     not be shared across goroutines.
type Repository[R any] interface {
	// WithTx returns a copy of the repository bound to the provided transaction.
	// The returned repository executes all database operations using the given
	// transaction context.
	//
	// This is typically called by BeginTxFunc to build a new repository instance
	// scoped to the lifetime of the transaction. Nested transactional calls
	// reuse the existing transaction, returning the original repository unchanged.
	WithTx(tx Tx) R

	// BeginTxFunc begins a new transaction (unless one is already in progress)
	// and executes fn with a tx-bound repository. The transaction is
	// automatically committed if fn returns nil, or rolled back if fn returns
	// an error or panics.
	//
	// Nested Behavior:
	//
	//   - If a transaction is already active (i.e. called within another
	//     BeginTxFunc or WithTx context), the existing transaction is reused
	//     and fn is invoked directly without starting a new transaction.
	//
	// Panic Semantics:
	//
	//   - If fn panics, the transaction helper will attempt to roll back and
	//     re-panic. If rollback itself fails, the panic message is annotated
	//     accordingly.
	//
	// Timeouts:
	//
	//   - Transactions should be configured with a timeout to avoid deadlocks.
	//     When timeout occurs, the returned error must be tagged with
	//     ErrTagTransactionTimeout.
	BeginTxFunc(ctx context.Context, fn func(ctx context.Context, tx Tx, repo R) error) error
}

func Do(ctx context.Context, tx Tx, fn func(ctx context.Context) error) error {
	defer func() {
		if r := recover(); r != nil {
			if rErr := tx.Rollback(ctx); rErr != nil {
				panic(fmt.Errorf("panic: %v; failed to rollback transaction: %w", r, rErr))
			}
			panic(r)
		}
	}()

	if err := fn(ctx); err != nil {
		if rErr := tx.Rollback(ctx); rErr != nil {
			err = fmt.Errorf("%w; failed to rollback transaction: %w", err, rErr)
		}
		return err
	}

	if cErr := tx.Commit(ctx); cErr != nil {
		return fmt.Errorf("failed to commit transaction: %w", cErr)
	}

	return nil
}
