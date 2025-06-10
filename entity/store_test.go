package entity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/encrypt"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/testutil"
)

func TestStore_Namespace(t *testing.T) {
	ctx := context.Background()
	s, _ := NewTestStore(t)

	want := NewNamespace(testutil.RandName())

	// create
	err := s.CreateNamespace(ctx, want)
	require.NoError(t, err)

	// read
	got, err := s.ReadNamespaceByName(ctx, want.Name)
	require.NoError(t, err)
	require.Equal(t, want, got)

	// delete
	err = s.DeleteNamespace(ctx, want.ID)
	require.NoError(t, err)

	_, err = s.ReadNamespaceByName(ctx, want.Name)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestStore_Operator(t *testing.T) {
	ctx := context.Background()
	s, _ := NewTestStore(t)

	want, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)

	// create
	err = s.CreateOperator(ctx, want)
	require.NoError(t, err)

	// read
	got, err := s.ReadOperator(ctx, want.ID)
	require.NoError(t, err)
	require.Equal(t, want, got)

	opData, err := want.Data()
	require.NoError(t, err)
	wantPubKey := opData.PublicKey

	got, err = s.ReadOperatorByPublicKey(ctx, wantPubKey)
	require.NoError(t, err)
	require.Equal(t, want, got)

	// update
	err = s.UpdateOperator(ctx, want)
	require.NoError(t, err)

	got, err = s.ReadOperator(ctx, want.ID)
	require.NoError(t, err)
	require.Equal(t, want, got)

	// delete
	err = s.DeleteOperator(ctx, want.ID)
	require.NoError(t, err)

	_, err = s.ReadOperator(ctx, want.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestStore_Account(t *testing.T) {
	ctx := context.Background()
	s, _ := NewTestStore(t)

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)

	want, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)

	// create
	err = s.CreateAccount(ctx, want)
	require.NoError(t, err)

	// read
	got, err := s.ReadAccount(ctx, want.ID)
	require.NoError(t, err)
	assertEqualAccount(t, want, got)

	wantData, err := want.Data()
	require.NoError(t, err)

	got, err = s.ReadAccountByPublicKey(ctx, wantData.PublicKey)
	require.NoError(t, err)
	assertEqualAccount(t, want, got)

	// update
	err = want.Update(op, UpdateAccountParams{
		Subscriptions: 10,
	})
	require.NoError(t, err)

	err = s.UpdateAccount(ctx, want)
	require.NoError(t, err)

	got, err = s.ReadAccount(ctx, want.ID)
	require.NoError(t, err)
	assertEqualAccount(t, want, got)

	// delete
	err = s.DeleteAccount(ctx, want.ID)
	require.NoError(t, err)

	_, err = s.ReadAccount(ctx, want.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestStore_User(t *testing.T) {
	ctx := context.Background()
	s, _ := NewTestStore(t)

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)

	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)

	sysAcc, err := NewSystemAccount(op)
	require.NoError(t, err)

	want, err := NewUser(testutil.RandName(), acc)
	require.NoError(t, err)

	wantSysUser, err := NewSystemUser(sysAcc)
	require.NoError(t, err)

	// create
	err = s.CreateUser(ctx, want)
	require.NoError(t, err)

	err = s.CreateUser(ctx, wantSysUser) // sys user
	require.NoError(t, err)

	// read
	got, err := s.ReadUser(ctx, want.ID)
	require.NoError(t, err)
	assertEqualUser(t, want, got)

	// read system user
	gotSysUser, err := s.ReadSystemUser(ctx, op.ID, sysAcc.ID)
	require.NoError(t, err)
	assertEqualUser(t, wantSysUser, gotSysUser)

	// delete
	err = s.DeleteUser(ctx, want.ID)
	require.NoError(t, err)

	_, err = s.ReadUser(ctx, want.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestStore_Nkey(t *testing.T) {
	ctx := context.Background()
	s, _ := NewTestStore(t)

	want, err := NewNkey(testutil.RandString(10), TypeOperator, false)
	require.NoError(t, err)

	err = s.CreateNkey(ctx, want)
	require.NoError(t, err)

	got, err := s.ReadNkey(ctx, want.ID, false)
	require.NoError(t, err)

	assert.Equal(t, want, got)
}

func NewTestStore(t *testing.T) (*Store, *FakeEntityRepository) {
	enc, err := encrypt.NewAES(testutil.RandString(32))
	require.NoError(t, err)

	rw := NewFakeEntityRepository(t)
	txer := new(testutil.FakeTxer)

	return NewStore(txer, rw, WithEncryption(enc)), rw
}
