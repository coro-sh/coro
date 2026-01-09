package tkn

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/joshjon/kit/id"
	"github.com/stretchr/testify/suite"

	"github.com/joshjon/kit/errtag"
	"github.com/joshjon/kit/testutil"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/tx"
)

const defaultTimeout = 5 * time.Second

type OperatorTokenReadWriterTestConfig struct {
	EntityRepo      entity.Repository
	OperatorTokenRW OperatorTokenReadWriter
}

// OperatorTokenReadWriterTestSuite verifies that an OperatorTokenReadWriter
// implementation satisfies the expected behavior required by the application.
// All tests must pass for an implementation to be considered compliant.
type OperatorTokenReadWriterTestSuite struct {
	// Timeout defines the maximum duration for each test (default: 5s).
	Timeout time.Duration

	// Setup is called before every test and must return a valid configuration
	// to use within each test run.
	Setup func(t *testing.T) OperatorTokenReadWriterTestConfig

	// SetTransactionTimeout sets the transaction timeout on the
	// OperatorTokenReadWriter. It is used only when testing transactions.
	SetTransactionTimeout func(t *testing.T, timeout time.Duration, rw OperatorTokenReadWriter)

	entityRepo entity.Repository
	opTknRW    OperatorTokenReadWriter
	suite.Suite
}

func (s *OperatorTokenReadWriterTestSuite) SetupTest() {
	s.Require().NotNil(s.Setup, "Setup func required")
	s.Require().NotNil(s.SetTransactionTimeout, "SetTransactionTimeout func required")

	cfg := s.Setup(s.T())
	s.Require().NotNil(cfg.EntityRepo, "EntityRepo must not be nil")
	s.Require().NotNil(cfg.OperatorTokenRW, "OperatorTokenRW must not be nil")
	s.entityRepo = cfg.EntityRepo
	s.opTknRW = cfg.OperatorTokenRW

	if s.Timeout == 0 {
		s.Timeout = defaultTimeout
	}
}

func (s *OperatorTokenReadWriterTestSuite) TestWriteReadToken() {
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.entityRepo.CreateNamespace(ctx, ns))

	opData := genOperatorData(ns)
	s.Require().NoError(s.entityRepo.CreateOperator(ctx, opData))

	wantTkn := testutil.RandString(30)

	err := s.opTknRW.Write(ctx, OperatorTokenTypeProxy, opData.ID, wantTkn)
	s.Require().NoError(err)

	got, err := s.opTknRW.Read(ctx, OperatorTokenTypeProxy, opData.ID)
	s.Require().NoError(err)
	s.Equal(wantTkn, got)
}

func (s *OperatorTokenReadWriterTestSuite) TestWritePerformsUpsertWithoutConflict() {
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.entityRepo.CreateNamespace(ctx, ns))

	opData := genOperatorData(ns)
	s.Require().NoError(s.entityRepo.CreateOperator(ctx, opData))

	err := s.opTknRW.Write(ctx, OperatorTokenTypeProxy, opData.ID, testutil.RandString(30))
	s.Require().NoError(err)

	wantTkn := testutil.RandString(30)

	err = s.opTknRW.Write(ctx, OperatorTokenTypeProxy, opData.ID, wantTkn)
	s.Require().NoError(err)

	got, err := s.opTknRW.Read(ctx, OperatorTokenTypeProxy, opData.ID)
	s.Require().NoError(err)
	s.Equal(wantTkn, got)
}

func (s *OperatorTokenReadWriterTestSuite) TestBeginTxFunc() {
	s.Run("commit tx", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		s.Require().NoError(s.entityRepo.CreateNamespace(ctx, ns))

		opData := genOperatorData(ns)
		s.Require().NoError(s.entityRepo.CreateOperator(ctx, opData))

		wantTkn := testutil.RandString(30)

		err := s.opTknRW.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo OperatorTokenReadWriter) error {
			return repo.Write(ctx, OperatorTokenTypeProxy, opData.ID, wantTkn)
		})
		s.Require().NoError(err)

		got, err := s.opTknRW.Read(ctx, OperatorTokenTypeProxy, opData.ID)
		s.Require().NoError(err)
		s.Equal(wantTkn, got)
	})

	s.Run("rollback tx on error", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		s.Require().NoError(s.entityRepo.CreateNamespace(ctx, ns))

		opData := genOperatorData(ns)
		s.Require().NoError(s.entityRepo.CreateOperator(ctx, opData))

		wantTkn := testutil.RandString(30)

		err := s.opTknRW.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo OperatorTokenReadWriter) error {
			s.Require().NoError(repo.Write(ctx, OperatorTokenTypeProxy, opData.ID, wantTkn))
			return errors.New("forced failure to trigger rollback")
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "forced failure")

		got, err := s.opTknRW.Read(ctx, OperatorTokenTypeProxy, opData.ID)
		s.Error(err)
		s.True(errtag.HasTag[errtag.NotFound](err))
		s.Empty(got)
	})

	s.Run("rollback tx on panic", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		s.Require().NoError(s.entityRepo.CreateNamespace(ctx, ns))

		opData := genOperatorData(ns)
		s.Require().NoError(s.entityRepo.CreateOperator(ctx, opData))

		wantTkn := testutil.RandString(30)

		func() {
			defer func() {
				if r := recover(); r != nil {
					s.T().Logf("recovered panic: %v", r)
				} else {
					s.Require().Fail("expected panic but none occurred")
				}
			}()

			_ = s.opTknRW.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo OperatorTokenReadWriter) error {
				s.Require().NoError(repo.Write(ctx, OperatorTokenTypeProxy, opData.ID, wantTkn))
				panic("forced panic to trigger rollback")
			})
		}()

		got, err := s.opTknRW.Read(ctx, OperatorTokenTypeProxy, opData.ID)
		s.Error(err)
		s.True(errtag.HasTag[errtag.NotFound](err))
		s.Empty(got)
	})

	s.Run("use same tx from entity repo tx", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		opData := genOperatorData(ns)

		wantTkn := testutil.RandString(30)

		err := s.entityRepo.BeginTxFunc(ctx, func(ctx context.Context, txn tx.Tx, txRepo entity.Repository) error {
			s.Require().NoError(txRepo.CreateNamespace(ctx, ns))
			s.Require().NoError(txRepo.CreateOperator(ctx, opData))

			txOpTknRw := s.opTknRW.WithTx(txn)
			s.Require().NoError(txOpTknRw.Write(ctx, OperatorTokenTypeProxy, opData.ID, wantTkn))

			return errors.New("forced failure to trigger rollback on both repos")
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "forced failure")

		_, err = s.entityRepo.ReadNamespace(ctx, ns.ID)
		s.Error(err)
		s.True(errtag.HasTag[entity.ErrTagNotFound[entity.Namespace]](err))

		_, err = s.entityRepo.ReadOperator(ctx, opData.ID)
		s.Error(err)
		s.True(errtag.HasTag[entity.ErrTagNotFound[entity.Operator]](err))

		got, err := s.opTknRW.Read(ctx, OperatorTokenTypeProxy, opData.ID)
		s.Error(err)
		s.True(errtag.HasTag[errtag.NotFound](err))
		s.Empty(got)
	})

	s.Run("tx timeout on next statement", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		timeout := 100 * time.Millisecond
		s.SetTransactionTimeout(s.T(), timeout, s.opTknRW)

		ns := genNamespace()
		s.Require().NoError(s.entityRepo.CreateNamespace(ctx, ns))

		opData := genOperatorData(ns)
		s.Require().NoError(s.entityRepo.CreateOperator(ctx, opData))

		err := s.opTknRW.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo OperatorTokenReadWriter) error {
			s.Require().NoError(repo.Write(ctx, OperatorTokenTypeProxy, opData.ID, testutil.RandString(30)))
			time.Sleep(timeout + (50 * time.Millisecond))

			// Next statement should fail due to the transaction timeout
			opID2 := id.New[entity.OperatorID]()
			err := repo.Write(ctx, OperatorTokenTypeProxy, opID2, testutil.RandString(30))
			s.Require().Error(err)
			s.True(errtag.HasTag[tx.ErrTagTransactionTimeout](err))
			return err
		})
		s.Require().Error(err)
		s.True(errtag.HasTag[tx.ErrTagTransactionTimeout](err))

		// Assert prior insert did not commit
		_, err = s.opTknRW.Read(ctx, OperatorTokenTypeProxy, opData.ID)
		s.Require().Error(err)
		s.True(errtag.HasTag[errtag.NotFound](err))
	})

	s.Run("tx timeout when idle", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		timeout := 100 * time.Millisecond
		s.SetTransactionTimeout(s.T(), timeout, s.opTknRW)

		ns := genNamespace()
		s.Require().NoError(s.entityRepo.CreateNamespace(ctx, ns))

		opData := genOperatorData(ns)
		s.Require().NoError(s.entityRepo.CreateOperator(ctx, opData))

		err := s.opTknRW.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo OperatorTokenReadWriter) error {
			s.Require().NoError(repo.Write(ctx, OperatorTokenTypeProxy, opData.ID, testutil.RandString(30)))
			time.Sleep(timeout + (50 * time.Millisecond))
			return nil
		})
		s.Require().Error(err)
		s.True(errtag.HasTag[tx.ErrTagTransactionTimeout](err))

		// Assert prior insert did not commit
		_, err = s.opTknRW.Read(ctx, OperatorTokenTypeProxy, opData.ID)
		s.Require().Error(err)
		s.True(errtag.HasTag[errtag.NotFound](err))
	})
}

func (s *OperatorTokenReadWriterTestSuite) TestErrorTags() {
	s.Run("operator token not found", func() {
		ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
		defer cancel()

		_, err := s.opTknRW.Read(ctx, OperatorTokenTypeProxy, id.New[entity.OperatorID]())
		s.Require().Error(err)
		s.True(errtag.HasTag[errtag.NotFound](err))
	})
}

func genNamespace() *entity.Namespace {
	return entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
}

func genOperatorData(ns *entity.Namespace) entity.OperatorData {
	return entity.OperatorData{
		OperatorIdentity: entity.OperatorIdentity{
			ID:          id.New[entity.OperatorID](),
			NamespaceID: ns.ID,
			JWT:         testutil.RandString(50),
		},
		Name:      testutil.RandName(),
		PublicKey: testutil.RandString(50),
	}
}
