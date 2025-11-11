package entity

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/ref"
	"github.com/coro-sh/coro/testutil"
)

func TestNewUser(t *testing.T) {
	op, err := NewOperator("test_operator", NewID[NamespaceID]())
	require.NoError(t, err)
	acc, err := NewAccount("test_account", op)
	require.NoError(t, err)
	accSKPubKey, err := acc.SigningKey().KeyPair().PublicKey()
	require.NoError(t, err)

	name := "test_user"
	user, err := NewUser(name, acc)
	require.NoError(t, err)
	AssertNewUser(t, accSKPubKey, name, user)
	isSysUser, err := user.IsSystemUser()
	require.NoError(t, err)
	assert.False(t, isSysUser)
}

func TestNewSystemUser(t *testing.T) {
	op, err := NewOperator("test_operator", NewID[NamespaceID]())
	require.NoError(t, err)

	t.Run("valid system user", func(t *testing.T) {
		acc, err := NewSystemAccount(op)
		require.NoError(t, err)
		accSKPubKey, err := acc.SigningKey().KeyPair().PublicKey()
		require.NoError(t, err)

		user, err := NewSystemUser(acc)
		require.NoError(t, err)
		AssertNewUser(t, accSKPubKey, constants.SysUserName, user)
		gotIsSys, err := user.IsSystemUser()
		require.NoError(t, err)
		assert.True(t, gotIsSys)
	})

	t.Run("non system account", func(t *testing.T) {
		account, err := NewAccount("non_system_account", op)
		require.NoError(t, err)
		_, err = NewSystemUser(account)
		require.EqualError(t, err, "not a system account")
	})
}

func TestNewUserFromData(t *testing.T) {
	operator, err := NewOperator("test_operator", NewID[NamespaceID]())
	require.NoError(t, err)

	account, err := NewAccount("test_account", operator)
	require.NoError(t, err)

	name := "test_user"
	user, err := NewUser(name, account)
	require.NoError(t, err)

	t.Run("valid user data", func(t *testing.T) {
		data, err := user.Data()
		require.NoError(t, err)
		got, err := NewUserFromData(user.NKey(), data)
		require.NoError(t, err)
		AssertEqualUser(t, user, got)
	})

	t.Run("invalid jwt in user data", func(t *testing.T) {
		data, err := user.Data()
		require.NoError(t, err)
		data.JWT = "invalid"
		_, err = NewUserFromData(user.NKey(), data)
		require.Error(t, err)
	})
}

func TestUser_Validate(t *testing.T) {
	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	user, err := NewUser(testutil.RandName(), acc)
	require.NoError(t, err)

	t.Run("valid user", func(t *testing.T) {
		err := user.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid claims", func(t *testing.T) {
		user.jwt = "invalid"
		err := user.Validate()
		assert.Error(t, err)
	})
}

func TestUser_Update(t *testing.T) {
	op, err := NewOperator("test_operator", NewID[NamespaceID]())
	require.NoError(t, err)
	acc, err := NewAccount("test_account", op)
	require.NoError(t, err)
	user, err := NewUser("test_user", acc)
	require.NoError(t, err)

	updateParams := UpdateUserParams{
		Subscriptions: 10,
		PayloadSize:   1024,
		JWTDuration:   ref.Ptr(time.Second),
	}

	err = user.Update(acc, updateParams)
	assert.NoError(t, err)

	claims, err := user.Claims()
	require.NoError(t, err)
	assert.Equal(t, updateParams.Subscriptions, claims.Limits.Subs)
	assert.Equal(t, updateParams.PayloadSize, claims.Limits.Payload)
	assert.Equal(t, updateParams.JWTDuration, user.jwtDuration)
}

func TestUser_RefreshJWT(t *testing.T) {
	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)

	user, err := NewUser(testutil.RandName(), acc)
	require.NoError(t, err)

	oldJWT := user.JWT()
	oldClaims, err := user.Claims()
	require.NoError(t, err)

	user.jwtDuration = ref.Ptr(time.Minute)
	newJWT, newClaims, err := user.RefreshJWT(acc)
	require.NoError(t, err)

	assert.NotEqual(t, oldJWT, newJWT)
	assert.Greater(t, newClaims.Expires, oldClaims.Expires)
}

func AssertNewUser(t *testing.T, wantIssuer string, wantName string, user *User) {
	t.Helper()
	assert.False(t, user.ID.IsZero())
	assert.False(t, user.OperatorID.IsZero())
	assert.False(t, user.AccountID.IsZero())
	assert.NotEmpty(t, user.JWT)
	assert.NotNil(t, user.NKey())

	claims, err := user.Claims()
	require.NoError(t, err)
	assert.Equal(t, wantIssuer, claims.Issuer)
	assert.Equal(t, wantName, claims.Name)

	nkPubKey, err := user.NKey().KeyPair().PublicKey()
	require.NoError(t, err)
	assert.Equal(t, nkPubKey, claims.Subject)

	data, err := user.Data()
	require.NoError(t, err)
	assert.Equal(t, user.ID, data.ID)
	assert.Equal(t, user.OperatorID, data.OperatorID)
	assert.Equal(t, user.AccountID, data.AccountID)
	assert.Equal(t, wantName, data.Name)
	assert.Equal(t, user.JWT(), data.JWT)
}

// Decoding user claims results in different zero values for some fields
// compared to when the claims were originally created e.g. empty value for
// UserLimits.Src decodes to jwt.CIDRList(nil) instead of jwt.CIDRList{}.
// To Work around this we assert fields individually and compare a JSON
// stringified version of the account claims instead.
func AssertEqualUser(t *testing.T, got *User, want *User) {
	t.Helper()
	wantData, err := got.Data()
	require.NoError(t, err)
	gotData, err := got.Data()
	require.NoError(t, err)
	assert.Equal(t, wantData, gotData)

	assert.Equal(t, want.NKey(), got.NKey())

	wantIsSys, err := want.IsSystemUser()
	require.NoError(t, err)
	gotIsSys, err := got.IsSystemUser()
	require.NoError(t, err)
	assert.Equal(t, wantIsSys, gotIsSys)

	wantClaims, err := want.Claims()
	require.NoError(t, err)
	wantClaimsJSON, err := json.Marshal(wantClaims)
	require.NoError(t, err)

	gotClaims, err := got.Claims()
	require.NoError(t, err)
	gotClaimsJSON, err := json.Marshal(gotClaims)
	require.NoError(t, err)

	assert.JSONEq(t, string(wantClaimsJSON), string(gotClaimsJSON))
}
