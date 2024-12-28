package tx

import (
	"context"
	"fmt"
)

type Txer interface {
	BeginTx(ctx context.Context) (Tx, error)
}

type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Handle is intended for deferred execution to correctly handle a transaction if
// a failure occurs. Arg `err` must a pointer to the error returned by the caller.
func Handle(ctx context.Context, tx Tx, err *error) {
	if r := recover(); r != nil {
		if rErr := tx.Rollback(ctx); rErr != nil {
			panic(fmt.Errorf("panic: %v; failed to rollback transaction: %w", r, rErr))
		}
		panic(r)
	}

	if err != nil {
		baseErr := *err
		if baseErr != nil {
			if rErr := tx.Rollback(ctx); rErr != nil {
				*err = fmt.Errorf("%w; failed to rollback transaction: %w", baseErr, rErr)
			}
			return
		}
	}

	if cErr := tx.Commit(ctx); cErr != nil {
		*err = fmt.Errorf("failed to commit transaction: %w", cErr)
	}
}
