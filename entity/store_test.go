package entity_test // avoid import cycle with sqlite package

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/sqlite"
	"github.com/coro-sh/coro/testutil"
)

func TestStore_Namespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	s, _ := sqlite.NewTestEntityStore(t)

	want := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)

	// create
	err := s.CreateNamespace(ctx, want)
	require.NoError(t, err)

	// read
	got, err := s.ReadNamespaceByName(ctx, want.Name, constants.DefaultNamespaceOwner)
	require.NoError(t, err)
	require.Equal(t, want, got)

	// delete
	err = s.DeleteNamespace(ctx, want.ID)
	require.NoError(t, err)

	_, err = s.ReadNamespaceByName(ctx, want.Name, constants.DefaultNamespaceOwner)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestStore_Operator(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	s, _ := sqlite.NewTestEntityStore(t)

	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	err := s.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	want, err := entity.NewOperator(testutil.RandName(), ns.ID)
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
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	s, _ := sqlite.NewTestEntityStore(t)

	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	err := s.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	require.NoError(t, err)
	err = s.CreateOperator(ctx, op)
	require.NoError(t, err)

	want, err := entity.NewAccount(testutil.RandName(), op)
	require.NoError(t, err)

	// create
	err = s.CreateAccount(ctx, want)
	require.NoError(t, err)

	// read
	got, err := s.ReadAccount(ctx, want.ID)
	require.NoError(t, err)
	entity.AssertEqualAccount(t, want, got)

	wantData, err := want.Data()
	require.NoError(t, err)

	got, err = s.ReadAccountByPublicKey(ctx, wantData.PublicKey)
	require.NoError(t, err)
	entity.AssertEqualAccount(t, want, got)

	// update
	err = want.Update(op, entity.UpdateAccountParams{
		Subscriptions: 10,
	})
	require.NoError(t, err)

	err = s.UpdateAccount(ctx, want)
	require.NoError(t, err)

	got, err = s.ReadAccount(ctx, want.ID)
	require.NoError(t, err)
	entity.AssertEqualAccount(t, want, got)

	// delete
	err = s.DeleteAccount(ctx, want.ID)
	require.NoError(t, err)

	_, err = s.ReadAccount(ctx, want.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestStore_User(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	s, _ := sqlite.NewTestEntityStore(t)

	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	err := s.CreateNamespace(ctx, ns)
	require.NoError(t, err)

	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	require.NoError(t, err)
	err = s.CreateOperator(ctx, op)
	require.NoError(t, err)

	acc, err := entity.NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	err = s.CreateAccount(ctx, acc)
	require.NoError(t, err)

	sysAcc, err := entity.NewSystemAccount(op)
	require.NoError(t, err)
	err = s.CreateAccount(ctx, sysAcc)
	require.NoError(t, err)

	want, err := entity.NewUser(testutil.RandName(), acc)
	require.NoError(t, err)

	wantSysUser, err := entity.NewSystemUser(sysAcc)
	require.NoError(t, err)

	// create
	err = s.CreateUser(ctx, want)
	require.NoError(t, err)

	err = s.CreateUser(ctx, wantSysUser) // sys user
	require.NoError(t, err)

	// read
	got, err := s.ReadUser(ctx, want.ID)
	require.NoError(t, err)
	entity.AssertEqualUser(t, want, got)

	// read system user
	gotSysUser, err := s.ReadSystemUser(ctx, op.ID, sysAcc.ID)
	require.NoError(t, err)
	entity.AssertEqualUser(t, wantSysUser, gotSysUser)

	// delete
	err = s.DeleteUser(ctx, want.ID)
	require.NoError(t, err)

	_, err = s.ReadUser(ctx, want.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestStore_Nkey(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	s, _ := sqlite.NewTestEntityStore(t)

	want, err := entity.NewNkey(testutil.RandString(10), entity.TypeOperator, false)
	require.NoError(t, err)

	err = s.CreateNkey(ctx, want)
	require.NoError(t, err)

	got, err := s.ReadNkey(ctx, want.ID, false)
	require.NoError(t, err)

	assert.Equal(t, want, got)
}
