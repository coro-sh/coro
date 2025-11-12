package sqlite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coro-sh/coro/entity"
)

func TestEntityRepositoryTestSuite(t *testing.T) {
	suite.Run(t, &entity.RepositoryTestSuite{
		Setup: func(t *testing.T) entity.Repository {
			return NewEntityRepository(NewTestDB(t))
		},
		SetTransactionTimeout: func(t *testing.T, timeout time.Duration, repo entity.Repository) {
			repo.(*EntityRepository).txer.Config.Timeout = timeout
		},
	})
}
