package entity

import (
	"context"
	"errors"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/paginate"
	"github.com/coro-sh/coro/ref"
	"github.com/coro-sh/coro/testutil"
	"github.com/coro-sh/coro/tx"
)

const defaultTestSuiteTimeout = 5 * time.Second

// RepositoryTestSuite verifies that a Repository implementation satisfies
// the expected behavior required by the application. All tests must pass
// for an implementation to be considered compliant.
type RepositoryTestSuite struct {
	// Timeout defines the maximum duration for each test (default: 5s).
	Timeout time.Duration

	// Setup is called before every test and must return a valid repository
	// to use within each test run. The repository transaction timeout in seconds
	// must be configured with param txTimeoutSeconds.
	Setup func(t *testing.T) Repository

	// SetTransactionTimeout sets the transaction timeout on the Repository.
	// It is used only when testing transactions.
	SetTransactionTimeout func(t *testing.T, timeout time.Duration, repo Repository)

	repo Repository
	suite.Suite
}

func (s *RepositoryTestSuite) SetupTest() {
	s.Require().NotNil(s.Setup, "Setup func required")
	s.Require().NotNil(s.SetTransactionTimeout, "SetTransactionTimeout func required")

	repo := s.Setup(s.T())
	s.Require().NotNil(repo, "Repository must not be nil")
	s.repo = repo

	if s.Timeout == 0 {
		s.Timeout = defaultTestSuiteTimeout
	}
}

func (s *RepositoryTestSuite) TestCreateReadNamespace() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	want := genNamespace()

	err := s.repo.CreateNamespace(ctx, want)
	s.Require().NoError(err)

	got, err := s.repo.ReadNamespace(ctx, want.ID)
	s.Require().NoError(err)
	s.Equal(want, got)

	got, err = s.repo.ReadNamespaceByName(ctx, want.Name, want.Owner)
	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *RepositoryTestSuite) TestListNamespaces() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	pageSize := int32(10)
	numNamespaces := 15
	wantNamespaces := make([]*Namespace, numNamespaces)

	// Reverse order since we fetch in descending order
	for i := numNamespaces - 1; i >= 0; i-- {
		data := genNamespace()
		err := s.repo.CreateNamespace(ctx, data)
		s.Require().NoError(err)
		wantNamespaces[i] = data
	}

	// First page
	filter := paginate.PageFilter[NamespaceID]{Size: pageSize}
	gotPage1, err := s.repo.ListNamespaces(ctx, constants.DefaultNamespaceOwner, filter)
	s.Require().NoError(err)
	s.Len(gotPage1, int(pageSize))
	wantPage1 := wantNamespaces[:pageSize]
	s.Equal(wantPage1, gotPage1)

	// Second page
	filter.Cursor = &gotPage1[len(gotPage1)-1].ID
	gotPage2, err := s.repo.ListNamespaces(ctx, constants.DefaultNamespaceOwner, filter)
	s.Require().NoError(err)
	// Note: cursor is included to support peeking the next page
	wantLenPage2 := numNamespaces - int(pageSize) + 1
	s.Len(gotPage2, wantLenPage2)
	s.Equal(wantNamespaces[pageSize-1:], gotPage2)
}

func (s *RepositoryTestSuite) TestDeleteNamespace() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	// setup entities
	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	acc := genAccountData(op)
	s.Require().NoError(s.repo.CreateAccount(ctx, acc))

	usr := genUserData(acc)
	s.Require().NoError(s.repo.CreateUser(ctx, usr))
	s.Require().NoError(s.repo.CreateUserJWTIssuance(ctx, usr.ID, UserJWTIssuance{IssueTime: time.Now().Unix()}))

	// delete namespace
	s.Require().NoError(s.repo.DeleteNamespace(ctx, ns.ID))
	assertNamespaceDeleted(s.T(), s.repo, ns.ID)

	// cascade delete
	assertOperatorDeleted(s.T(), s.repo, op.ID)
	assertAccountDeleted(s.T(), s.repo, acc.ID)
	assertUserDeleted(s.T(), s.repo, usr.ID)
}

func (s *RepositoryTestSuite) TestCreateReadOperator() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	want := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, want))

	got, err := s.repo.ReadOperator(ctx, want.ID)
	s.Require().NoError(err)
	s.Equal(want, got)

	got, err = s.repo.ReadOperatorByPublicKey(ctx, want.PublicKey)
	s.Require().NoError(err)
	s.Equal(want, got)

	got, err = s.repo.ReadOperatorByName(ctx, want.Name)
	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *RepositoryTestSuite) TestUpdateOperator() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	want := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, want))

	got, err := s.repo.ReadOperator(ctx, want.ID)
	s.Require().NoError(err)
	s.Equal(want, got)

	want.Name = "foo"
	want.JWT = "bar"
	s.Require().NoError(s.repo.UpdateOperator(ctx, want))

	got, err = s.repo.ReadOperator(ctx, want.ID)
	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *RepositoryTestSuite) TestListOperators() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	pageSize := int32(10)
	numOps := 15
	wantOps := make([]OperatorData, numOps)

	// Reverse order since we fetch in descending order
	for i := numOps - 1; i >= 0; i-- {
		data := genOperatorData(ns)
		s.Require().NoError(s.repo.CreateOperator(ctx, data))
		wantOps[i] = data
	}

	// First page
	filter := paginate.PageFilter[OperatorID]{Size: pageSize}
	gotPage1, err := s.repo.ListOperators(ctx, ns.ID, filter)
	s.Require().NoError(err)
	s.Len(gotPage1, int(pageSize))
	wantPage1 := wantOps[:pageSize]
	s.Equal(wantPage1, gotPage1)

	// Second page
	filter.Cursor = &gotPage1[len(gotPage1)-1].ID
	gotPage2, err := s.repo.ListOperators(ctx, ns.ID, filter)
	s.Require().NoError(err)
	wantLenPage2 := numOps - int(pageSize) + 1
	s.Len(gotPage2, wantLenPage2)
	s.Equal(wantOps[pageSize-1:], gotPage2)
}

func (s *RepositoryTestSuite) TestDeleteOperator() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	// setup entities
	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	acc := genAccountData(op)
	s.Require().NoError(s.repo.CreateAccount(ctx, acc))

	usr := genUserData(acc)
	s.Require().NoError(s.repo.CreateUser(ctx, usr))
	s.Require().NoError(s.repo.CreateUserJWTIssuance(ctx, usr.ID, UserJWTIssuance{IssueTime: time.Now().Unix()}))

	// delete operator
	s.Require().NoError(s.repo.DeleteOperator(ctx, op.ID))
	assertOperatorDeleted(s.T(), s.repo, op.ID)

	// cascade delete
	assertAccountDeleted(s.T(), s.repo, acc.ID)
	assertUserDeleted(s.T(), s.repo, usr.ID)
}

func (s *RepositoryTestSuite) TestCreateReadAccount() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	want := genAccountData(op)
	s.Require().NoError(s.repo.CreateAccount(ctx, want))

	got, err := s.repo.ReadAccount(ctx, want.ID)
	s.Require().NoError(err)
	s.Equal(want, got)

	got, err = s.repo.ReadAccountByPublicKey(ctx, want.PublicKey)
	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *RepositoryTestSuite) TestUpdateAccount() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	want := genAccountData(op)
	s.Require().NoError(s.repo.CreateAccount(ctx, want))

	got, err := s.repo.ReadAccount(ctx, want.ID)
	s.Require().NoError(err)
	s.Equal(want, got)

	want.Name = "foo"
	want.JWT = "bar"
	*want.UserJWTDuration = time.Hour * time.Duration(290*365*24) // 290 years

	s.Require().NoError(s.repo.UpdateAccount(ctx, want))

	got, err = s.repo.ReadAccount(ctx, want.ID)
	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *RepositoryTestSuite) TestListAccounts() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	pageSize := int32(10)
	numAccs := 15
	wantAccs := make([]AccountData, numAccs)

	// Reverse order since we fetch in descending order
	for i := numAccs - 1; i >= 0; i-- {
		data := genAccountData(op)
		s.Require().NoError(s.repo.CreateAccount(ctx, data))
		wantAccs[i] = data
	}

	// First page
	filter := paginate.PageFilter[AccountID]{Size: pageSize}
	gotPage1, err := s.repo.ListAccounts(ctx, op.ID, filter)
	s.Require().NoError(err)
	s.Len(gotPage1, int(pageSize))
	wantPage1 := wantAccs[:pageSize]
	s.Equal(wantPage1, gotPage1)

	// Second page
	filter.Cursor = &gotPage1[len(gotPage1)-1].ID
	gotPage2, err := s.repo.ListAccounts(ctx, op.ID, filter)
	s.Require().NoError(err)
	wantLenPage2 := numAccs - int(pageSize) + 1
	s.Len(gotPage2, wantLenPage2)
	s.Equal(wantAccs[pageSize-1:], gotPage2)
}

func (s *RepositoryTestSuite) TestDeleteAccount() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	// setup entities
	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	acc := genAccountData(op)
	s.Require().NoError(s.repo.CreateAccount(ctx, acc))

	usr := genUserData(acc)
	s.Require().NoError(s.repo.CreateUser(ctx, usr))
	s.Require().NoError(s.repo.CreateUserJWTIssuance(ctx, usr.ID, UserJWTIssuance{IssueTime: time.Now().Unix()}))

	// delete account
	s.Require().NoError(s.repo.DeleteAccount(ctx, acc.ID))
	assertAccountDeleted(s.T(), s.repo, acc.ID)

	// cascade delete
	assertUserDeleted(s.T(), s.repo, usr.ID)
}

func (s *RepositoryTestSuite) TestCreateReadUser() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	acc := genAccountData(op)
	s.Require().NoError(s.repo.CreateAccount(ctx, acc))

	want := genUserData(acc)
	s.Require().NoError(s.repo.CreateUser(ctx, want))

	got, err := s.repo.ReadUser(ctx, want.ID)
	s.Require().NoError(err)
	s.Equal(want, got)

	got, err = s.repo.ReadUserByName(ctx, op.ID, acc.ID, want.Name)
	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *RepositoryTestSuite) TestUpdateUser() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	acc := genAccountData(op)
	s.Require().NoError(s.repo.CreateAccount(ctx, acc))

	want := genUserData(acc)
	s.Require().NoError(s.repo.CreateUser(ctx, want))

	got, err := s.repo.ReadUser(ctx, want.ID)
	s.Require().NoError(err)
	s.Equal(want, got)

	want.Name = "foo"
	want.JWT = "bar"
	*want.JWTDuration = time.Hour * time.Duration(290*365*24) // 290 years

	s.Require().NoError(s.repo.UpdateUser(ctx, want))

	got, err = s.repo.ReadUser(ctx, want.ID)
	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *RepositoryTestSuite) TestListUsers() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	acc := genAccountData(op)
	s.Require().NoError(s.repo.CreateAccount(ctx, acc))

	pageSize := int32(10)
	numUsers := 15
	wantUsers := make([]UserData, numUsers)

	// Reverse order since we fetch in descending order
	for i := numUsers - 1; i >= 0; i-- {
		data := genUserData(acc)
		s.Require().NoError(s.repo.CreateUser(ctx, data))
		wantUsers[i] = data
	}

	// First page
	filter := paginate.PageFilter[UserID]{Size: pageSize}
	gotPage1, err := s.repo.ListUsers(ctx, acc.ID, filter)
	s.Require().NoError(err)
	s.Len(gotPage1, int(pageSize))
	wantPage1 := wantUsers[:pageSize]
	s.Equal(wantPage1, gotPage1)

	// Second page
	filter.Cursor = &gotPage1[len(gotPage1)-1].ID
	gotPage2, err := s.repo.ListUsers(ctx, acc.ID, filter)
	s.Require().NoError(err)
	wantLenPage2 := numUsers - int(pageSize) + 1
	s.Len(gotPage2, wantLenPage2)
	s.Equal(wantUsers[pageSize-1:], gotPage2)
}

func (s *RepositoryTestSuite) TestDeleteUser() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	// setup entities
	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	acc := genAccountData(op)
	s.Require().NoError(s.repo.CreateAccount(ctx, acc))

	usr := genUserData(acc)
	s.Require().NoError(s.repo.CreateUser(ctx, usr))
	s.Require().NoError(s.repo.CreateUserJWTIssuance(ctx, usr.ID, UserJWTIssuance{IssueTime: time.Now().Unix()}))

	// delete user
	s.Require().NoError(s.repo.DeleteUser(ctx, usr.ID))
	assertUserDeleted(s.T(), s.repo, usr.ID)
}

func (s *RepositoryTestSuite) TestCountOwnerNamespaces() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	wantCount := 20

	for i := 0; i < wantCount; i++ {
		ns := genNamespace()
		err := s.repo.CreateNamespace(ctx, ns)
		s.Require().NoError(err)
	}

	// create namespace with a different owner (should be excluded from count)
	otherNS := genNamespace()
	otherNS.Owner = testutil.RandName()
	err := s.repo.CreateNamespace(ctx, otherNS)

	got, err := s.repo.CountOwnerNamespaces(ctx, constants.DefaultNamespaceOwner)
	s.Require().NoError(err)
	s.Equal(int64(wantCount), got)
}

func (s *RepositoryTestSuite) TestCountOwnerOperators() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	wantCount := 20

	ns := genNamespace()
	err := s.repo.CreateNamespace(ctx, ns)
	s.Require().NoError(err)

	for i := 0; i < wantCount; i++ {
		op := genOperatorData(ns)
		err = s.repo.CreateOperator(ctx, op)
		s.Require().NoError(err)
	}

	// create operator in a different namespace/owner (should be excluded from count)
	otherNS := genNamespace()
	otherNS.Owner = testutil.RandName()
	s.Require().NoError(s.repo.CreateNamespace(ctx, otherNS))
	s.Require().NoError(s.repo.CreateOperator(ctx, genOperatorData(otherNS)))

	got, err := s.repo.CountOwnerOperators(ctx, constants.DefaultNamespaceOwner)
	s.Require().NoError(err)
	s.Equal(int64(wantCount), got)
}

func (s *RepositoryTestSuite) TestCountOwnerAccounts() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	wantCount := 20

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))
	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))

	for i := 0; i < wantCount; i++ {
		acc := genAccountData(op)
		s.Require().NoError(s.repo.CreateAccount(ctx, acc))
	}

	// create account in a different namespace/owner (should be excluded from count)
	otherNS := genNamespace()
	otherNS.Owner = testutil.RandName()
	s.Require().NoError(s.repo.CreateNamespace(ctx, otherNS))
	otherOp := genOperatorData(otherNS)
	s.Require().NoError(s.repo.CreateOperator(ctx, otherOp))
	s.Require().NoError(s.repo.CreateAccount(ctx, genAccountData(otherOp)))

	got, err := s.repo.CountOwnerAccounts(ctx, constants.DefaultNamespaceOwner)
	s.Require().NoError(err)
	s.Equal(int64(wantCount), got)
}

func (s *RepositoryTestSuite) TestCountOwnerUsers() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	wantCount := 20

	ns := genNamespace()
	s.Require().NoError(s.repo.CreateNamespace(ctx, ns))
	op := genOperatorData(ns)
	s.Require().NoError(s.repo.CreateOperator(ctx, op))
	acc := genAccountData(op)
	s.Require().NoError(s.repo.CreateAccount(ctx, acc))

	for i := 0; i < wantCount; i++ {
		usr := genUserData(acc)
		s.Require().NoError(s.repo.CreateUser(ctx, usr))
	}

	// create users in a different namespace/owner (should be excluded from count)
	otherNS := genNamespace()
	otherNS.Owner = testutil.RandName()
	s.Require().NoError(s.repo.CreateNamespace(ctx, otherNS))
	otherOp := genOperatorData(otherNS)
	s.Require().NoError(s.repo.CreateOperator(ctx, otherOp))
	otherAcc := genAccountData(otherOp)
	s.Require().NoError(s.repo.CreateAccount(ctx, otherAcc))
	s.Require().NoError(s.repo.CreateUser(ctx, genUserData(otherAcc)))

	got, err := s.repo.CountOwnerUsers(ctx, constants.DefaultNamespaceOwner)
	s.Require().NoError(err)
	s.Equal(int64(wantCount), got)
}

func (s *RepositoryTestSuite) TestCreateReadNkey() {
	ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
	defer cancel()

	want := genNkeyData()

	s.Require().NoError(s.repo.CreateNkey(ctx, want))

	got, err := s.repo.ReadNkey(ctx, want.ID, want.SigningKey)
	s.Require().NoError(err)
	s.Equal(want, got)

	// Signing key
	want.SigningKey = true
	s.Require().NoError(s.repo.CreateNkey(ctx, want))

	got, err = s.repo.ReadNkey(ctx, want.ID, want.SigningKey)
	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *RepositoryTestSuite) TestBeginTxFunc() {
	s.Run("commit tx", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		err := s.repo.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo Repository) error {
			return repo.CreateNamespace(ctx, ns)
		})
		s.Require().NoError(err)

		got, err := s.repo.ReadNamespace(ctx, ns.ID)
		s.Require().NoError(err)
		s.Equal(ns, got)
	})

	s.Run("rollback tx on error", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		err := s.repo.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo Repository) error {
			s.Require().NoError(repo.CreateNamespace(ctx, ns))
			return errors.New("forced error to trigger rollback")
		})
		s.Require().Error(err)
		s.Contains(err.Error(), "forced error")

		got, err := s.repo.ReadNamespace(ctx, ns.ID)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Namespace]](err))
		s.Nil(got)
	})

	s.Run("rollback tx on panic", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		defer func() {
			if r := recover(); r != nil {
				s.T().Logf("recovered panic: %v", r)
			} else {
				s.Require().Fail("expected panic but none occurred")
			}
		}()

		_ = s.repo.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo Repository) error {
			s.Require().NoError(repo.CreateNamespace(ctx, ns))
			panic("forced panic to trigger rollback")
		})

		got, err := s.repo.ReadNamespace(ctx, ns.ID)
		s.Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Namespace]](err))
		s.Nil(got)
	})

	s.Run("tx timeout on next statement", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		timeout := 100 * time.Millisecond
		s.SetTransactionTimeout(s.T(), timeout, s.repo)

		ns := genNamespace()
		err := s.repo.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo Repository) error {
			s.Require().NoError(repo.CreateNamespace(ctx, ns))
			time.Sleep(timeout + (50 * time.Millisecond))

			// Next statement should fail due to the transaction timeout
			_, err := repo.CountOwnerNamespaces(ctx, constants.DefaultNamespaceOwner)
			s.Require().Error(err)
			s.True(errtag.HasTag[tx.ErrTagTransactionTimeout](err))
			return err
		})
		s.Require().Error(err)
		s.True(errtag.HasTag[tx.ErrTagTransactionTimeout](err))

		// Assert prior insert did not commit
		_, err = s.repo.ReadNamespace(ctx, ns.ID)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Namespace]](err))
	})

	s.Run("tx timeout when idle", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		timeout := 100 * time.Millisecond
		s.SetTransactionTimeout(s.T(), timeout, s.repo)

		ns := genNamespace()
		err := s.repo.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo Repository) error {
			s.Require().NoError(repo.CreateNamespace(ctx, ns))
			time.Sleep(timeout + (50 * time.Millisecond))
			return nil
		})
		s.Require().Error(err)
		s.True(errtag.HasTag[tx.ErrTagTransactionTimeout](err))

		// Assert prior insert did not commit
		_, err = s.repo.ReadNamespace(ctx, ns.ID)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Namespace]](err))
	})
}

func (s *RepositoryTestSuite) TestErrorTags() {
	s.Run("namespace not found", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		_, err := s.repo.ReadNamespace(ctx, NewID[NamespaceID]())
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Namespace]](err))

		_, err = s.repo.ReadNamespaceByName(ctx, testutil.RandName(), constants.DefaultNamespaceOwner)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Namespace]](err))
	})

	s.Run("namespace create conflict - duplicate (name, owner)", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

		dup := NewNamespace(ns.Name, ns.Owner)
		err := s.repo.CreateNamespace(ctx, dup)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagConflict[Namespace]](err))
	})

	s.Run("operator not found", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		_, err := s.repo.ReadOperator(ctx, NewID[OperatorID]())
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Operator]](err))

		_, err = s.repo.ReadOperatorByPublicKey(ctx, testutil.RandString(24))
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Operator]](err))

		_, err = s.repo.ReadOperatorByName(ctx, testutil.RandName())
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Operator]](err))
	})

	s.Run("operator create conflict - duplicate id", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

		op := genOperatorData(ns)
		s.Require().NoError(s.repo.CreateOperator(ctx, op))

		err := s.repo.CreateOperator(ctx, op)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagConflict[Operator]](err))
	})

	s.Run("account not found", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		_, err := s.repo.ReadAccount(ctx, NewID[AccountID]())
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Account]](err))

		_, err = s.repo.ReadAccountByPublicKey(ctx, testutil.RandString(24))
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[Account]](err))
	})

	s.Run("account create conflict - duplicate id ", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

		op := genOperatorData(ns)
		s.Require().NoError(s.repo.CreateOperator(ctx, op))

		acc := genAccountData(op)
		s.Require().NoError(s.repo.CreateAccount(ctx, acc))

		err := s.repo.CreateAccount(ctx, acc)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagConflict[Account]](err))
	})

	s.Run("user not found", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		_, err := s.repo.ReadUser(ctx, NewID[UserID]())
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[User]](err))

		ns := genNamespace()
		s.Require().NoError(s.repo.CreateNamespace(ctx, ns))

		op := genOperatorData(ns)
		s.Require().NoError(s.repo.CreateOperator(ctx, op))

		acc := genAccountData(op)
		s.Require().NoError(s.repo.CreateAccount(ctx, acc))

		_, err = s.repo.ReadUserByName(ctx, op.ID, acc.ID, testutil.RandName())
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNotFound[User]](err))
	})

	s.Run("user create conflict - duplicate id", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		ns := genNamespace()
		s.Require().NoError(s.repo.CreateNamespace(ctx, ns))
		op := genOperatorData(ns)
		s.Require().NoError(s.repo.CreateOperator(ctx, op))
		acc := genAccountData(op)
		s.Require().NoError(s.repo.CreateAccount(ctx, acc))
		u := genUserData(acc)
		s.Require().NoError(s.repo.CreateUser(ctx, u))
		err := s.repo.CreateUser(ctx, u)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagConflict[User]](err))
	})

	s.Run("nkey not found", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		_, err := s.repo.ReadNkey(ctx, testutil.RandString(10), false)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNkeyNotFound](err))
	})

	s.Run("signing key not found", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		_, err := s.repo.ReadNkey(ctx, testutil.RandString(10), true)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagSigningKeyNotFound](err))
	})

	s.Run("nkey conflict - duplicate id", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		nk := genNkeyData()
		s.Require().NoError(s.repo.CreateNkey(ctx, nk))

		err := s.repo.CreateNkey(ctx, nk)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagNkeyConflict](err))
	})

	s.Run("signing key conflict - duplicate id", func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), s.Timeout)
		defer cancel()

		nk := genNkeyData()
		s.Require().NoError(s.repo.CreateNkey(ctx, nk))

		sk := nk
		sk.SigningKey = true
		s.Require().NoError(s.repo.CreateNkey(ctx, sk))

		err := s.repo.CreateNkey(ctx, sk)
		s.Require().Error(err)
		s.True(errtag.HasTag[ErrTagSigningKeyConflict](err))
	})
}

func genNamespace() *Namespace {
	return NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
}

func genOperatorData(ns *Namespace) OperatorData {
	return OperatorData{
		OperatorIdentity: OperatorIdentity{
			ID:          NewID[OperatorID](),
			NamespaceID: ns.ID,
			JWT:         testutil.RandString(50),
		},
		Name:      testutil.RandName(),
		PublicKey: testutil.RandString(50),
	}
}

func genAccountData(op OperatorData) AccountData {
	return AccountData{
		AccountIdentity: AccountIdentity{
			ID:          NewID[AccountID](),
			NamespaceID: op.NamespaceID,
			OperatorID:  op.ID,
			JWT:         testutil.RandString(50),
		},
		Name:            testutil.RandName(),
		PublicKey:       testutil.RandString(50),
		UserJWTDuration: ref.Ptr(time.Second * time.Duration(rand.IntN(1000))),
	}
}

func genUserData(acc AccountData) UserData {
	return UserData{
		UserIdentity: UserIdentity{
			ID:          NewID[UserID](),
			NamespaceID: acc.NamespaceID,
			OperatorID:  acc.OperatorID,
			AccountID:   acc.ID,
		},
		Name:        testutil.RandName(),
		JWT:         testutil.RandString(50),
		JWTDuration: ref.Ptr(time.Second * time.Duration(rand.IntN(1000))),
	}
}

func genNkeyData() NkeyData {
	return NkeyData{
		ID:         testutil.RandString(10),
		Type:       TypeUser,
		Seed:       []byte("test_seed_1"),
		SigningKey: false,
	}
}

func assertNamespaceDeleted(t *testing.T, repo Repository, nsID NamespaceID) {
	t.Helper()
	_, err := repo.ReadNamespace(t.Context(), nsID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func assertOperatorDeleted(t *testing.T, repo Repository, opID OperatorID) {
	t.Helper()
	_, err := repo.ReadOperator(t.Context(), opID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
	assertNkeysDeleted(t, repo, opID.String())
}

func assertAccountDeleted(t *testing.T, repo Repository, accID AccountID) {
	t.Helper()
	_, err := repo.ReadAccount(t.Context(), accID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
	assertNkeysDeleted(t, repo, accID.String())
}

func assertUserDeleted(t *testing.T, repo Repository, userID UserID) {
	t.Helper()
	_, err := repo.ReadUser(t.Context(), userID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
	assertNkeysDeleted(t, repo, userID.String())
	gotUserIss, err := repo.ListUserJWTIssuances(t.Context(), userID, paginate.PageFilter[int64]{Size: 1})
	assert.NoError(t, err)
	assert.Len(t, gotUserIss, 0)
}

func assertNkeysDeleted(t *testing.T, repo Repository, entityID string) {
	t.Helper()
	_, err := repo.ReadNkey(t.Context(), entityID, false)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
	_, err = repo.ReadNkey(t.Context(), entityID, true)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}
