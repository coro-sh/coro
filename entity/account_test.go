package entity

import (
	"encoding/json"
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/constants"
	"github.com/joshjon/kit/testutil"
)

func TestNewAccount(t *testing.T) {
	op, err := NewOperator("test_operator", id.New[NamespaceID]())
	require.NoError(t, err)
	opSKPubKey, err := op.SigningKey().KeyPair().PublicKey()
	require.NoError(t, err)

	name := "test_account"
	got, err := NewAccount(name, op)
	require.NoError(t, err)
	AssertNewAccount(t, opSKPubKey, name, got)
	gotIsSys, err := got.IsSystemAccount()
	require.NoError(t, err)
	assert.False(t, gotIsSys)
}

func TestNewSystemAccount(t *testing.T) {
	op, err := NewOperator("test_operator", id.New[NamespaceID]())
	require.NoError(t, err)
	opSKPubKey, err := op.SigningKey().KeyPair().PublicKey()
	require.NoError(t, err)

	got, err := NewSystemAccount(op)
	require.NoError(t, err)
	AssertNewAccount(t, opSKPubKey, constants.SysAccountName, got)
	gotIsSys, err := got.IsSystemAccount()
	require.NoError(t, err)
	assert.True(t, gotIsSys)
}

func TestNewAccountFromData(t *testing.T) {
	op, err := NewOperator("test_operator", id.New[NamespaceID]())
	require.NoError(t, err)

	name := "test_account"
	acc, err := NewAccount(name, op)
	require.NoError(t, err)

	t.Run("valid account data", func(t *testing.T) {
		data, err := acc.Data()
		require.NoError(t, err)
		got, err := NewAccountFromData(acc.NKey(), acc.SigningKey(), data)
		require.NoError(t, err)
		AssertEqualAccount(t, acc, got)
	})

	t.Run("invalid jwt in account data", func(t *testing.T) {
		data, err := acc.Data()
		require.NoError(t, err)
		data.JWT = "invalid"
		_, err = NewAccountFromData(acc.NKey(), acc.SigningKey(), data)
		require.Error(t, err)
	})
}

func TestAccount_Update(t *testing.T) {
	op, err := NewOperator("test_operator", id.New[NamespaceID]())
	require.NoError(t, err)

	acc, err := NewAccount("test_account", op)
	require.NoError(t, err)

	updateParams := UpdateAccountParams{
		Subscriptions: 10,
		PayloadSize:   1024,
		Imports:       5,
		Exports:       3,
		Connections:   100,
	}

	err = acc.Update(op, updateParams)
	require.NoError(t, err)

	claims, err := acc.Claims()
	require.NoError(t, err)

	assert.Equal(t, updateParams.Subscriptions, claims.Limits.Subs)
	assert.Equal(t, updateParams.PayloadSize, claims.Limits.Payload)
	assert.Equal(t, updateParams.Imports, claims.Limits.Imports)
	assert.Equal(t, updateParams.Exports, claims.Limits.Exports)
	assert.Equal(t, updateParams.Connections, claims.Limits.Conn)
}

func TestAccount_Validate(t *testing.T) {
	op, err := NewOperator(testutil.RandName(), id.New[NamespaceID]())
	require.NoError(t, err)

	tests := []struct {
		name    string
		modify  func(*Account)
		isValid bool
	}{
		{
			name:    "valid account",
			modify:  func(acc *Account) {},
			isValid: true,
		},
		{
			name: "invalid nkey",
			modify: func(acc *Account) {
				acc.nk.Type = TypeOperator
			},
		},
		{
			name: "invalid signing key",
			modify: func(acc *Account) {
				acc.sk.Type = TypeOperator
			},
		},
		{
			name: "invalid claims",
			modify: func(acc *Account) {
				acc.JWT = "invalid"
			},
		},
		{
			name: "claims signing key mismatch",
			modify: func(acc *Account) {
				claims, err := jwt.DecodeAccountClaims(acc.JWT)
				require.NoError(t, err)
				claims.SigningKeys = jwt.SigningKeys{}
				acc.JWT, err = claims.Encode(op.SigningKey().KeyPair())
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, err := NewAccount(testutil.RandName(), op)
			require.NoError(t, err)

			tt.modify(acc)

			err = acc.Validate()
			if tt.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func AssertNewAccount(t *testing.T, wantIssuer string, wantName string, account *Account) {
	t.Helper()
	assert.False(t, account.ID.IsZero())
	assert.False(t, account.OperatorID.IsZero())
	assert.NotEmpty(t, account.JWT)
	assert.NotNil(t, account.NKey())
	assert.NotNil(t, account.SigningKey())

	claims, err := account.Claims()
	require.NoError(t, err)
	assert.Equal(t, wantIssuer, claims.Issuer)
	assert.Equal(t, wantName, claims.Name)

	nkPubKey, err := account.NKey().KeyPair().PublicKey()
	require.NoError(t, err)
	skPubKey, err := account.SigningKey().KeyPair().PublicKey()
	require.NoError(t, err)

	assert.Equal(t, nkPubKey, claims.Subject)
	assert.True(t, claims.SigningKeys.Contains(skPubKey))

	data, err := account.Data()
	require.NoError(t, err)
	assert.Equal(t, account.ID, data.ID)
	assert.Equal(t, account.OperatorID, data.OperatorID)
	assert.Equal(t, wantName, data.Name)
	assert.Equal(t, nkPubKey, data.PublicKey)
	assert.Equal(t, account.JWT, data.JWT)
}

// Decoding account claims results in different zero values for some fields
// compared to when the claims were originally created e.g. empty value for
// Account.Mappings decodes to jwt.Mapping(nil) instead of jwt.Mapping{}.
// To Work around this we assert fields individually and compare a JSON
// stringified version of the account claims instead.
func AssertEqualAccount(t *testing.T, want *Account, got *Account) {
	t.Helper()
	wantData, err := got.Data()
	require.NoError(t, err)
	gotData, err := got.Data()
	require.NoError(t, err)
	assert.Equal(t, wantData, gotData)

	assert.Equal(t, want.NKey(), got.NKey())
	assert.Equal(t, want.SigningKey(), got.SigningKey())

	wantIsSys, err := want.IsSystemAccount()
	require.NoError(t, err)
	gotIsSys, err := got.IsSystemAccount()
	require.NoError(t, err)
	assert.Equal(t, wantIsSys, gotIsSys)

	wantClaims, err := want.Claims()
	require.NoError(t, err)
	gotClaims, err := got.Claims()
	require.NoError(t, err)

	wantClaimsJSON, err := json.Marshal(wantClaims)
	assert.NoError(t, err)
	gotClaimsJSON, err := json.Marshal(gotClaims)
	assert.NoError(t, err)

	assert.JSONEq(t, string(wantClaimsJSON), string(gotClaimsJSON))
}
