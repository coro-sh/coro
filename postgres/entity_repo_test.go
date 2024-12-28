//go:build integration

package postgres

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/internal/testutil"
	"github.com/coro-sh/coro/postgres/migrations"
)

const timeout = 5 * time.Second

func TestEntityRepository_CreateReadNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	want := genNamespace()

	err := repo.CreateNamespace(ctx, want)
	require.NoError(t, err)

	got, err := repo.ReadNamespace(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	got, err = repo.ReadNamespaceByName(ctx, want.Name)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestEntityRepository_ListNamespaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	pageSize := int32(10)
	numNamespaces := 15
	wantNamespaces := make([]*entity.Namespace, numNamespaces)

	// Reverse order since we fetch in descending order
	for i := numNamespaces - 1; i >= 0; i-- {
		data := genNamespace()
		err := repo.CreateNamespace(ctx, data)
		require.NoError(t, err)
		wantNamespaces[i] = data
	}

	// First page
	filter := entity.PageFilter[entity.NamespaceID]{Size: pageSize}
	gotPage1, err := repo.ListNamespaces(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, gotPage1, int(pageSize))
	wantPage1 := wantNamespaces[:pageSize]
	assert.Equal(t, wantPage1, gotPage1)

	// Second page
	filter.Cursor = &gotPage1[len(gotPage1)-1].ID
	gotPage2, err := repo.ListNamespaces(ctx, filter)
	require.NoError(t, err)
	// Note: cursor is included to support peeking the next page
	wantLenPage2 := numNamespaces - int(pageSize) + 1
	assert.Len(t, gotPage2, wantLenPage2)
	assert.Equal(t, wantNamespaces[pageSize-1:], gotPage2)
}

func TestEntityRepository_DeleteNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	// setup entities
	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)
	op := genOperatorData(ns)
	err = repo.CreateOperator(ctx, op)
	require.NoError(t, err)
	acc := genAccountData(op)
	err = repo.CreateAccount(ctx, acc)
	require.NoError(t, err)
	usr := genUserData(acc)
	err = repo.CreateUser(ctx, usr)
	require.NoError(t, err)
	err = repo.CreateUserJWTIssuance(ctx, usr.ID, entity.UserJWTIssuance{IssueTime: time.Now().Unix()})
	require.NoError(t, err)

	// delete namespace
	err = repo.DeleteNamespace(ctx, ns.ID)
	require.NoError(t, err)
	assertNamespaceDeleted(t, repo, ns.ID)

	// cascade delete
	assertOperatorDeleted(t, repo, op.ID)
	assertAccountDeleted(t, repo, acc.ID)
	assertUserDeleted(t, repo, usr.ID)
}

func TestEntityRepository_CreateReadOperator(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	want := genOperatorData(ns)

	err = repo.CreateOperator(ctx, want)
	require.NoError(t, err)

	got, err := repo.ReadOperator(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	got, err = repo.ReadOperatorByPublicKey(ctx, want.PublicKey)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	got, err = repo.ReadOperatorByName(ctx, want.Name)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestEntityRepository_UpdateOperator(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	want := genOperatorData(ns)

	err = repo.CreateOperator(ctx, want)
	require.NoError(t, err)

	got, err := repo.ReadOperator(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	want.Name = "foo"
	want.JWT = "bar"

	err = repo.UpdateOperator(ctx, want)
	require.NoError(t, err)

	got, err = repo.ReadOperator(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestEntityRepository_ListOperators(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	pageSize := int32(10)
	numOps := 15
	wantOps := make([]entity.OperatorData, numOps)

	// Reverse order since we fetch in descending order
	for i := numOps - 1; i >= 0; i-- {
		data := genOperatorData(ns)
		err := repo.CreateOperator(ctx, data)
		require.NoError(t, err)
		wantOps[i] = data
	}

	// First page
	filter := entity.PageFilter[entity.OperatorID]{Size: pageSize}
	gotPage1, err := repo.ListOperators(ctx, ns.ID, filter)
	require.NoError(t, err)
	assert.Len(t, gotPage1, int(pageSize))
	wantPage1 := wantOps[:pageSize]
	assert.Equal(t, wantPage1, gotPage1)

	// Second page
	filter.Cursor = &gotPage1[len(gotPage1)-1].ID
	gotPage2, err := repo.ListOperators(ctx, ns.ID, filter)
	require.NoError(t, err)
	// Note: cursor is included to support peeking the next page
	wantLenPage2 := numOps - int(pageSize) + 1
	assert.Len(t, gotPage2, wantLenPage2)
	assert.Equal(t, wantOps[pageSize-1:], gotPage2)
}

func TestEntityRepository_DeleteOperator(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	// setup entities
	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)
	op := genOperatorData(ns)
	err = repo.CreateOperator(ctx, op)
	require.NoError(t, err)
	acc := genAccountData(op)
	err = repo.CreateAccount(ctx, acc)
	require.NoError(t, err)
	usr := genUserData(acc)
	err = repo.CreateUser(ctx, usr)
	require.NoError(t, err)
	err = repo.CreateUserJWTIssuance(ctx, usr.ID, entity.UserJWTIssuance{IssueTime: time.Now().Unix()})
	require.NoError(t, err)

	// delete operator
	err = repo.DeleteOperator(ctx, op.ID)
	require.NoError(t, err)
	assertOperatorDeleted(t, repo, op.ID)

	// cascade delete
	assertAccountDeleted(t, repo, acc.ID)
	assertUserDeleted(t, repo, usr.ID)
}

func TestEntityRepository_CreateReadAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	opData := genOperatorData(ns)
	err = repo.CreateOperator(ctx, opData)
	require.NoError(t, err)

	want := genAccountData(opData)

	err = repo.CreateAccount(ctx, want)
	require.NoError(t, err)

	got, err := repo.ReadAccount(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	got, err = repo.ReadAccountByPublicKey(ctx, want.PublicKey)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestEntityRepository_UpdateAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	opData := genOperatorData(ns)
	err = repo.CreateOperator(ctx, opData)
	require.NoError(t, err)

	want := genAccountData(opData)

	err = repo.CreateAccount(ctx, want)
	require.NoError(t, err)

	got, err := repo.ReadAccount(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	want.Name = "foo"
	want.JWT = "bar"
	*want.UserJWTDuration = time.Hour * time.Duration(290*365*24) // 290 years

	err = repo.UpdateAccount(ctx, want)
	require.NoError(t, err)

	got, err = repo.ReadAccount(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestEntityRepository_ListAccounts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	pageSize := int32(10)
	numAccs := 15
	wantAccs := make([]entity.AccountData, numAccs)

	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	op := genOperatorData(ns)
	err = repo.CreateOperator(ctx, op)
	require.NoError(t, err)

	// Reverse order since we fetch in descending order
	for i := numAccs - 1; i >= 0; i-- {
		data := genAccountData(op)
		err := repo.CreateAccount(ctx, data)
		require.NoError(t, err)
		wantAccs[i] = data
	}

	// First page
	filter := entity.PageFilter[entity.AccountID]{Size: pageSize}
	gotPage1, err := repo.ListAccounts(ctx, op.ID, filter)
	require.NoError(t, err)
	assert.Len(t, gotPage1, int(pageSize))
	wantPage1 := wantAccs[:pageSize]
	assert.Equal(t, wantPage1, gotPage1)

	// Second page
	filter.Cursor = &gotPage1[len(gotPage1)-1].ID
	gotPage2, err := repo.ListAccounts(ctx, op.ID, filter)
	require.NoError(t, err)
	// Note: cursor is included to support peeking the next page
	wantLenPage2 := numAccs - int(pageSize) + 1
	assert.Len(t, gotPage2, wantLenPage2)
	assert.Equal(t, wantAccs[pageSize-1:], gotPage2)
}

func TestEntityRepository_DeleteAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	// setup entities
	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)
	op := genOperatorData(ns)
	err = repo.CreateOperator(ctx, op)
	require.NoError(t, err)
	acc := genAccountData(op)
	err = repo.CreateAccount(ctx, acc)
	require.NoError(t, err)
	usr := genUserData(acc)
	err = repo.CreateUser(ctx, usr)
	require.NoError(t, err)
	err = repo.CreateUserJWTIssuance(ctx, usr.ID, entity.UserJWTIssuance{IssueTime: time.Now().Unix()})
	require.NoError(t, err)

	// delete account
	err = repo.DeleteAccount(ctx, acc.ID)
	require.NoError(t, err)
	assertAccountDeleted(t, repo, acc.ID)

	// cascade delete
	assertUserDeleted(t, repo, usr.ID)
}

func TestEntityRepository_CreateReadUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	opData := genOperatorData(ns)
	err = repo.CreateOperator(ctx, opData)
	require.NoError(t, err)

	accData := genAccountData(opData)
	err = repo.CreateAccount(ctx, accData)
	require.NoError(t, err)

	want := genUserData(accData)

	err = repo.CreateUser(ctx, want)
	require.NoError(t, err)

	got, err := repo.ReadUser(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	got, err = repo.ReadUserByName(ctx, opData.ID, accData.ID, want.Name)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestEntityRepository_UpdateUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	opData := genOperatorData(ns)
	err = repo.CreateOperator(ctx, opData)
	require.NoError(t, err)

	accData := genAccountData(opData)
	err = repo.CreateAccount(ctx, accData)
	require.NoError(t, err)

	want := genUserData(accData)

	err = repo.CreateUser(ctx, want)
	require.NoError(t, err)

	got, err := repo.ReadUser(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	want.Name = "foo"
	want.JWT = "bar"
	*want.JWTDuration = time.Hour * time.Duration(290*365*24) // 290 years

	err = repo.UpdateUser(ctx, want)
	require.NoError(t, err)

	got, err = repo.ReadUser(ctx, want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestEntityRepository_ListUsers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	pageSize := int32(10)
	numUsers := 15
	wantUsers := make([]entity.UserData, numUsers)

	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	op := genOperatorData(ns)
	err = repo.CreateOperator(ctx, op)
	require.NoError(t, err)

	acc := genAccountData(op)
	err = repo.CreateAccount(ctx, acc)
	require.NoError(t, err)

	// Reverse order since we fetch in descending order
	for i := numUsers - 1; i >= 0; i-- {
		data := genUserData(acc)
		err := repo.CreateUser(ctx, data)
		require.NoError(t, err)
		wantUsers[i] = data
	}

	// First page
	filter := entity.PageFilter[entity.UserID]{Size: pageSize}
	gotPage1, err := repo.ListUsers(ctx, acc.ID, filter)
	require.NoError(t, err)
	assert.Len(t, gotPage1, int(pageSize))
	wantPage1 := wantUsers[:pageSize]
	assert.Equal(t, wantPage1, gotPage1)

	// Second page
	filter.Cursor = &gotPage1[len(gotPage1)-1].ID
	gotPage2, err := repo.ListUsers(ctx, acc.ID, filter)
	require.NoError(t, err)
	// Note: cursor is included to support peeking the next page
	wantLenPage2 := numUsers - int(pageSize) + 1
	assert.Len(t, gotPage2, wantLenPage2)
	assert.Equal(t, wantUsers[pageSize-1:], gotPage2)
}

func TestEntityRepository_DeleteUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	// setup entities
	ns := genNamespace()
	err := repo.CreateNamespace(ctx, ns)
	require.NoError(t, err)
	op := genOperatorData(ns)
	err = repo.CreateOperator(ctx, op)
	require.NoError(t, err)
	acc := genAccountData(op)
	err = repo.CreateAccount(ctx, acc)
	require.NoError(t, err)
	usr := genUserData(acc)
	err = repo.CreateUser(ctx, usr)
	require.NoError(t, err)
	err = repo.CreateUserJWTIssuance(ctx, usr.ID, entity.UserJWTIssuance{IssueTime: time.Now().Unix()})
	require.NoError(t, err)

	// delete user
	err = repo.DeleteUser(ctx, usr.ID)
	require.NoError(t, err)
	assertUserDeleted(t, repo, usr.ID)
}

func TestEntityRepository_CreateReadNkey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db := setupTestDB(t)
	repo := NewEntityRepository(db)

	want := genNkeyData()

	err := repo.CreateNkey(ctx, want)
	require.NoError(t, err)

	got, err := repo.ReadNkey(ctx, want.ID, want.SigningKey)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	// Signing key

	want.SigningKey = true
	err = repo.CreateNkey(ctx, want)
	require.NoError(t, err)

	got, err = repo.ReadNkey(ctx, want.ID, want.SigningKey)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestEntityRepository_WithTx(t *testing.T) {
	tests := []struct {
		name       string
		commit     bool
		wantExists bool
	}{
		{
			name:       "commit tx",
			commit:     true,
			wantExists: true,
		},
		{
			name:       "rollback tx",
			commit:     false,
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			db := setupTestDB(t)
			txer := NewTxer(db)
			repo := NewEntityRepository(db)

			tx, err := txer.BeginTx(ctx)
			require.NoError(t, err)
			repoTx, err := repo.WithTx(tx)
			require.NoError(t, err)

			nk := genNkeyData()

			err = repoTx.CreateNkey(ctx, nk)
			require.NoError(t, err)

			if tt.commit {
				err = tx.Commit(ctx)
				require.NoError(t, err)

				got, err := repo.ReadNkey(ctx, nk.ID, nk.SigningKey)
				require.NoError(t, err)
				assert.NotEmpty(t, got)
			} else {
				err = tx.Rollback(ctx)
				require.NoError(t, err)

				got, err := repo.ReadNkey(ctx, nk.ID, nk.SigningKey)
				require.Error(t, err)
				require.True(t, errtag.HasTag[NkeyNotFound](err))
				assert.Empty(t, got)
			}
		})
	}
}

func genNamespace() *entity.Namespace {
	return entity.NewNamespace(testutil.RandName())
}

func genOperatorData(ns *entity.Namespace) entity.OperatorData {
	return entity.OperatorData{
		OperatorIdentity: entity.OperatorIdentity{
			ID:          entity.NewID[entity.OperatorID](),
			NamespaceID: ns.ID,
			JWT:         testutil.RandString(50),
		},
		Name:      testutil.RandName(),
		PublicKey: testutil.RandString(50),
	}
}

func genAccountData(op entity.OperatorData) entity.AccountData {
	return entity.AccountData{
		AccountIdentity: entity.AccountIdentity{
			ID:          entity.NewID[entity.AccountID](),
			NamespaceID: op.NamespaceID,
			OperatorID:  op.ID,
			JWT:         testutil.RandString(50),
		},
		Name:            testutil.RandName(),
		PublicKey:       testutil.RandString(50),
		UserJWTDuration: ptr(time.Second * time.Duration(rand.IntN(1000))),
	}
}

func genUserData(acc entity.AccountData) entity.UserData {
	return entity.UserData{
		UserIdentity: entity.UserIdentity{
			ID:          entity.NewID[entity.UserID](),
			NamespaceID: acc.NamespaceID,
			OperatorID:  acc.OperatorID,
			AccountID:   acc.ID,
		},
		Name:        testutil.RandName(),
		JWT:         testutil.RandString(50),
		JWTDuration: ptr(time.Second * time.Duration(rand.IntN(1000))),
	}
}

func genNkeyData() entity.NkeyData {
	return entity.NkeyData{
		ID:         testutil.RandString(10),
		Type:       entity.TypeUser,
		Seed:       []byte("test_seed_1"),
		SigningKey: false,
	}
}

func assertNamespaceDeleted(t *testing.T, repo *EntityRepository, nsID entity.NamespaceID) {
	_, err := repo.ReadNamespace(t.Context(), nsID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func assertOperatorDeleted(t *testing.T, repo *EntityRepository, opID entity.OperatorID) {
	_, err := repo.ReadOperator(t.Context(), opID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
	assertNkeysDeleted(t, repo, opID.String())
}

func assertAccountDeleted(t *testing.T, repo *EntityRepository, accID entity.AccountID) {
	_, err := repo.ReadAccount(t.Context(), accID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
	assertNkeysDeleted(t, repo, accID.String())
}

func assertUserDeleted(t *testing.T, repo *EntityRepository, userID entity.UserID) {
	_, err := repo.ReadUser(t.Context(), userID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
	assertNkeysDeleted(t, repo, userID.String())
	gotUserIss, err := repo.ListUserJWTIssuances(t.Context(), userID, entity.PageFilter[int64]{Size: 1})
	assert.NoError(t, err)
	assert.Len(t, gotUserIss, 0)
}

func assertNkeysDeleted(t *testing.T, repo *EntityRepository, entityID string) {
	_, err := repo.ReadNkey(t.Context(), entityID, false)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
	_, err = repo.ReadNkey(t.Context(), entityID, true)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func setupTestDB(t *testing.T) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	recreateDB(t)
	pool, err := Dial(ctx, "postgres", "postgres", "localhost:5432", AppDBName)
	require.NoError(t, err)

	err = migrations.MigrateDatabase(pool)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
		recreateDB(t)
	})

	return pool
}

func recreateDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pool, err := Dial(ctx, "postgres", "postgres", "localhost:5432", "postgres")
	require.NoError(t, err)
	err = pool.Ping(ctx)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DROP DATABASE IF EXISTS "+AppDBName)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE DATABASE "+AppDBName)
	require.NoError(t, err)
	pool.Close()
}
