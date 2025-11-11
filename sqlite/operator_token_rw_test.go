package sqlite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coro-sh/coro/tkn"
)

func TestOperatorTokenReadWriterTestSuite(t *testing.T) {
	suite.Run(t, &tkn.OperatorTokenReadWriterTestSuite{
		Setup: func(t *testing.T) tkn.OperatorTokenReadWriterTestConfig {
			db := setupTestDB(t)
			return tkn.OperatorTokenReadWriterTestConfig{
				EntityRepo:      NewEntityRepository(db),
				OperatorTokenRW: NewOperatorTokenReadWriter(db),
			}
		},
		SetTransactionTimeout: func(t *testing.T, timeout time.Duration, repo tkn.OperatorTokenReadWriter) {
			repo.(*OperatorTokenReadWriter).txer.Config.Timeout = timeout
		},
	})
}
