package entity

import (
	"context"
	"fmt"
	"testing"

	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/internal/testutil"
	"github.com/coro-sh/coro/tx"
)

var errFakeNotFound = errtag.NewTagged[errtag.NotFound]("fake not found")

var _ Repository = (*FakeEntityRepository)(nil)

type FakeEntityRepository struct {
	namespaces        *testutil.KV[NamespaceID, *Namespace]
	operators         *testutil.KV[OperatorID, OperatorData]
	accounts          *testutil.KV[AccountID, AccountData]
	users             *testutil.KV[UserID, UserData]
	userJwtIss        *testutil.KV[UserID, UserJWTIssuance]
	proxyTokens       *testutil.KV[OperatorID, string]
	internalOperators *testutil.KV[OperatorID, OperatorData]
	nkeys             *testutil.KV[string, NkeyData]
	signingKeys       *testutil.KV[string, NkeyData]
}

func NewFakeEntityRepository(t *testing.T) *FakeEntityRepository {
	t.Helper()
	return &FakeEntityRepository{
		namespaces: testutil.NewKV(t,
			testutil.WithIndex[NamespaceID, *Namespace]("name", func(ns *Namespace) string {
				return ns.Name
			}),
		),
		operators: testutil.NewKV(t,
			testutil.WithIndex[OperatorID, OperatorData]("pub_key", func(op OperatorData) string {
				return op.PublicKey
			}),
			testutil.WithIndex[OperatorID, OperatorData]("name", func(op OperatorData) string {
				return op.Name
			}),
		),
		accounts: testutil.NewKV(t,
			testutil.WithIndex[AccountID, AccountData]("pub_key", func(acc AccountData) string {
				return acc.PublicKey
			}),
		),
		users: testutil.NewKV(t,
			testutil.WithIndex[UserID, UserData]("name", func(user UserData) string {
				return userNameIndexKey(user.OperatorID, user.AccountID, user.Name)
			}),
		),
		userJwtIss:  testutil.NewKV[UserID, UserJWTIssuance](t),
		proxyTokens: testutil.NewKV[OperatorID, string](t),
		internalOperators: testutil.NewKV(t,
			testutil.WithIndex[OperatorID, OperatorData]("name", func(op OperatorData) string {
				return op.Name
			}),
		),
		nkeys:       testutil.NewKV[string, NkeyData](t),
		signingKeys: testutil.NewKV[string, NkeyData](t),
	}
}

func (r *FakeEntityRepository) CreateNamespace(_ context.Context, namespace *Namespace) error {
	r.namespaces.Put(namespace.ID, namespace)
	return nil
}

func (r *FakeEntityRepository) ReadNamespaceByName(_ context.Context, name string) (*Namespace, error) {
	ns, ok := r.namespaces.GetByIndex("name", name)
	if !ok {
		return nil, errFakeNotFound
	}
	return ns, nil
}

func (r *FakeEntityRepository) ListNamespaces(_ context.Context, filter PageFilter[NamespaceID]) ([]*Namespace, error) {
	kvFilter := testutil.PageFilter{
		Size:   filter.Size,
		Cursor: filter.Cursor,
	}
	return r.namespaces.List(kvFilter), nil
}

func (r *FakeEntityRepository) DeleteNamespace(ctx context.Context, id NamespaceID) error {
	if _, ok := r.namespaces.Get(id); !ok {
		return errFakeNotFound
	}

	r.operators.Range(func(operatorID OperatorID, op OperatorData) bool {
		if op.NamespaceID == id {
			_ = r.DeleteOperator(ctx, operatorID)
		}
		return true
	})

	r.nkeys.Delete(id.String())
	r.signingKeys.Delete(id.String())
	r.namespaces.Delete(id)
	return nil
}

func (r *FakeEntityRepository) CreateOperator(_ context.Context, operator OperatorData) error {
	r.operators.Put(operator.ID, operator)
	return nil
}

func (r *FakeEntityRepository) UpdateOperator(_ context.Context, operator OperatorData) error {
	existing, ok := r.operators.Get(operator.ID)
	if !ok {
		return errFakeNotFound
	}
	existing.Name = operator.Name
	existing.JWT = operator.JWT
	r.operators.Put(existing.ID, existing)
	return nil
}

func (r *FakeEntityRepository) ReadOperator(_ context.Context, id OperatorID) (OperatorData, error) {
	op, ok := r.operators.Get(id)
	if !ok {
		return OperatorData{}, errFakeNotFound
	}
	return op, nil
}

func (r *FakeEntityRepository) ReadOperatorByName(_ context.Context, name string) (OperatorData, error) {
	op, ok := r.operators.GetByIndex("name", name)
	if !ok {
		return OperatorData{}, errFakeNotFound
	}
	return op, nil
}

func (r *FakeEntityRepository) ReadOperatorByPublicKey(_ context.Context, pubKey string) (OperatorData, error) {
	op, ok := r.operators.GetByIndex("pub_key", pubKey)
	if !ok {
		return OperatorData{}, errFakeNotFound
	}
	return op, nil
}

func (r *FakeEntityRepository) DeleteOperator(ctx context.Context, id OperatorID) error {
	if _, ok := r.operators.Get(id); !ok {
		return errFakeNotFound
	}

	r.accounts.Range(func(accountID AccountID, acc AccountData) bool {
		if acc.OperatorID == id {
			_ = r.DeleteAccount(ctx, accountID)
		}
		return true
	})

	r.nkeys.Delete(id.String())
	r.signingKeys.Delete(id.String())
	r.operators.Delete(id)
	return nil
}

func (r *FakeEntityRepository) ListOperators(_ context.Context, namespaceID NamespaceID, filter PageFilter[OperatorID]) ([]OperatorData, error) {
	kvFilter := testutil.PageFilter{
		Size:   filter.Size,
		Cursor: filter.Cursor,
	}
	return r.operators.List(kvFilter, func(_ OperatorID, data OperatorData) bool {
		return data.NamespaceID != namespaceID
	}), nil
}

func (r *FakeEntityRepository) CreateAccount(_ context.Context, account AccountData) error {
	r.accounts.Put(account.ID, account)
	return nil
}

func (r *FakeEntityRepository) UpdateAccount(_ context.Context, account AccountData) error {
	existing, ok := r.accounts.Get(account.ID)
	if !ok {
		return errFakeNotFound
	}
	existing.Name = account.Name
	existing.JWT = account.JWT
	existing.UserJWTDuration = account.UserJWTDuration
	r.accounts.Put(existing.ID, existing)
	return nil
}

func (r *FakeEntityRepository) ReadAccount(_ context.Context, id AccountID) (AccountData, error) {
	acc, ok := r.accounts.Get(id)
	if !ok {
		return AccountData{}, errFakeNotFound
	}
	return acc, nil
}

func (r *FakeEntityRepository) ReadAccountByPublicKey(_ context.Context, pubKey string) (AccountData, error) {
	acc, ok := r.accounts.GetByIndex("pub_key", pubKey)
	if !ok {
		return AccountData{}, errFakeNotFound
	}
	return acc, nil
}

func (r *FakeEntityRepository) ListAccounts(_ context.Context, operatorID OperatorID, filter PageFilter[AccountID]) ([]AccountData, error) {
	kvFilter := testutil.PageFilter{
		Size:   filter.Size,
		Cursor: filter.Cursor,
	}
	return r.accounts.List(kvFilter, func(_ AccountID, data AccountData) bool {
		return data.OperatorID != operatorID
	}), nil
}

func (r *FakeEntityRepository) DeleteAccount(ctx context.Context, id AccountID) error {
	if _, ok := r.accounts.Get(id); !ok {
		return errFakeNotFound
	}

	r.users.Range(func(userID UserID, user UserData) bool {
		if user.AccountID == id {
			_ = r.DeleteUser(ctx, userID)
		}
		return true
	})

	r.nkeys.Delete(id.String())
	r.signingKeys.Delete(id.String())
	r.accounts.Delete(id)
	return nil
}

func (r *FakeEntityRepository) CreateUser(_ context.Context, user UserData) error {
	r.users.Put(user.ID, user)
	return nil
}

func (r *FakeEntityRepository) UpdateUser(_ context.Context, user UserData) error {
	existing, ok := r.users.Get(user.ID)
	if !ok {
		return errFakeNotFound
	}
	existing.Name = user.Name
	existing.JWT = user.JWT
	existing.JWTDuration = user.JWTDuration
	r.users.Put(existing.ID, existing)
	return nil
}

func (r *FakeEntityRepository) ReadUser(_ context.Context, id UserID) (UserData, error) {
	usr, ok := r.users.Get(id)
	if !ok {
		return UserData{}, errFakeNotFound
	}
	return usr, nil
}

func (r *FakeEntityRepository) ReadUserByName(_ context.Context, operatorID OperatorID, accountID AccountID, name string) (UserData, error) {
	usr, ok := r.users.GetByIndex("name", userNameIndexKey(operatorID, accountID, name))
	if !ok {
		return UserData{}, errFakeNotFound
	}
	return usr, nil
}

func (r *FakeEntityRepository) ListUsers(_ context.Context, accountID AccountID, filter PageFilter[UserID]) ([]UserData, error) {
	kvFilter := testutil.PageFilter{
		Size:   filter.Size,
		Cursor: filter.Cursor,
	}
	return r.users.List(kvFilter, func(_ UserID, data UserData) bool {
		return data.AccountID != accountID
	}), nil
}

func (r *FakeEntityRepository) DeleteUser(_ context.Context, id UserID) error {
	if _, ok := r.users.Get(id); !ok {
		return errFakeNotFound
	}

	r.userJwtIss.Range(func(userID UserID, _ UserJWTIssuance) bool {
		if user, ok := r.users.Get(userID); ok && user.ID == id {
			r.userJwtIss.Delete(userID)
		}
		return true
	})

	r.nkeys.Delete(id.String())
	r.signingKeys.Delete(id.String())
	r.users.Delete(id)
	return nil
}

func (r *FakeEntityRepository) CreateUserJWTIssuance(_ context.Context, userID UserID, iss UserJWTIssuance) error {
	r.userJwtIss.Put(userID, UserJWTIssuance{
		IssueTime:  iss.IssueTime,
		ExpireTime: iss.ExpireTime,
	})
	return nil
}

func (r *FakeEntityRepository) ListUserJWTIssuances(_ context.Context, userID UserID, filter PageFilter[int64]) ([]UserJWTIssuance, error) {
	kvFilter := testutil.PageFilter{
		Size:   filter.Size,
		Cursor: filter.Cursor,
	}
	items := r.userJwtIss.List(kvFilter, func(key UserID, _ UserJWTIssuance) bool {
		return key != userID
	})
	return items, nil
}

func (r *FakeEntityRepository) CreateInternalOperator(_ context.Context, operator OperatorData) error {
	r.operators.Put(operator.ID, operator)
	return nil
}

func (r *FakeEntityRepository) ReadInternalOperatorByName(_ context.Context, name string) (OperatorData, error) {
	op, ok := r.operators.GetByIndex("name", name)
	if !ok {
		return OperatorData{}, errFakeNotFound
	}
	return op, nil
}

func (r *FakeEntityRepository) CreateNkey(_ context.Context, nkey NkeyData) error {
	if nkey.SigningKey {
		r.signingKeys.Put(nkey.ID, nkey)
		return nil
	}
	r.nkeys.Put(nkey.ID, nkey)
	return nil
}

func (r *FakeEntityRepository) ReadNkey(_ context.Context, id string, signingKey bool) (NkeyData, error) {
	if signingKey {
		sk, ok := r.signingKeys.Get(id)
		if !ok {
			return NkeyData{}, errFakeNotFound
		}
		return sk, nil
	}
	nk, ok := r.nkeys.Get(id)
	if !ok {
		return NkeyData{}, errFakeNotFound
	}
	return nk, nil
}

func (r *FakeEntityRepository) WithTx(_ tx.Tx) (Repository, error) {
	return r, nil
}

func userNameIndexKey(operatorID OperatorID, accountID AccountID, name string) string {
	return fmt.Sprintf("%s#%s#%s", operatorID, accountID, name)
}
