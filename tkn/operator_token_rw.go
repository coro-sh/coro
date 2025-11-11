package tkn

import (
	"context"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/tx"
)

// OperatorTokenReadWriter is the interface for performing Read and Write
// operations on Operator tokens.
//
// Implementations must pass all tests in the OperatorTokenReadWriterTestSuite
// to be considered compliant for use in the application.
type OperatorTokenReadWriter interface {
	Read(ctx context.Context, tokenType OperatorTokenType, operatorID entity.OperatorID) (string, error)
	Write(ctx context.Context, tokenType OperatorTokenType, operatorID entity.OperatorID, hashedToken string) error
	tx.Repository[OperatorTokenReadWriter]
}
