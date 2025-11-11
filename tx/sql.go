package tx

import (
	"context"
	"database/sql"
)

// SQLTxWrapper adapts *sql.Tx to the Tx interface used by Do. It allows
// database/sql transactions to integrate with generic commit/rollback helpers
// without leaking driver-specific APIs.
type SQLTxWrapper struct {
	base *sql.Tx
}

// NewSQLTxWrapper wraps an *sql.Tx to satisfy the Tx interface.
func NewSQLTxWrapper(tx *sql.Tx) *SQLTxWrapper {
	return &SQLTxWrapper{base: tx}
}

// Commit commits the underlying SQL transaction.
func (s *SQLTxWrapper) Commit(ctx context.Context) error {
	done := make(chan error, 1)
	go func() { done <- s.base.Commit() }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Context expired before commit finished so try to rollback
		_ = s.base.Rollback() // ignore err since it may already be closed
		return ctx.Err()
	}
}

// Rollback rolls back the underlying SQL transaction.
func (s *SQLTxWrapper) Rollback(_ context.Context) error {
	return s.base.Rollback()
}

// GetSQLTx returns the underlying *sql.Tx. This is primarily used internally by
// repository binders (the withTx function) to rebind sqlc.Queries or other
// database handles.
func (s *SQLTxWrapper) GetSQLTx() *sql.Tx {
	return s.base
}
