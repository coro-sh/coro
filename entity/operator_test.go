package entity

import (
	"testing"

	"github.com/joshjon/kit/id"
	"github.com/nats-io/jwt/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/kit/testutil"
)

func TestNewOperator(t *testing.T) {
	name := testutil.RandName()
	got, err := NewOperator(name, id.New[NamespaceID]())
	require.NoError(t, err)

	assert.False(t, got.ID.IsZero())
	assert.NotEmpty(t, got.JWT)
	assert.NotNil(t, got.NKey())
	assert.NotNil(t, got.SigningKey())

	claims, err := got.Claims()
	require.NoError(t, err)
	assert.True(t, claims.StrictSigningKeyUsage)
	assert.Equal(t, name, claims.Name)

	nkPubKey, err := got.NKey().KeyPair().PublicKey()
	require.NoError(t, err)
	assert.Equal(t, nkPubKey, claims.Subject)

	skPubKey, err := got.SigningKey().KeyPair().PublicKey()
	require.NoError(t, err)
	assert.True(t, claims.SigningKeys.Contains(skPubKey))

	data, err := got.Data()
	require.NoError(t, err)
	assert.Equal(t, name, data.Name)
	assert.NotEmpty(t, claims.Subject)
}

func TestNewOperatorFromJWT(t *testing.T) {
	operator, err := NewOperator(testutil.RandName(), id.New[NamespaceID]())
	require.NoError(t, err)

	t.Run("valid data", func(t *testing.T) {
		data, err := operator.Data()
		require.NoError(t, err)
		got, err := NewOperatorFromData(operator.NKey(), operator.SigningKey(), data)
		require.NoError(t, err)
		require.Equal(t, operator, got)
	})

	t.Run("invalid jwt", func(t *testing.T) {
		data, err := operator.Data()
		require.NoError(t, err)
		data.JWT = "invalid"
		_, err = NewOperatorFromData(operator.NKey(), operator.SigningKey(), data)
		require.Error(t, err)
	})
}

func TestOperator_SetName(t *testing.T) {
	op, err := NewOperator(testutil.RandName(), id.New[NamespaceID]())
	require.NoError(t, err)

	err = op.SetName("foo")
	require.NoError(t, err)

	data, err := op.Data()
	require.NoError(t, err)
	assert.Equal(t, "foo", data.Name)
}

func TestOperator_SetSystemAccount(t *testing.T) {
	op, err := NewOperator(testutil.RandName(), id.New[NamespaceID]())
	require.NoError(t, err)

	acc, err := NewAccount("test_account", op)
	require.NoError(t, err)
	accData, err := acc.Data()
	require.NoError(t, err)

	err = op.SetSystemAccount(acc)
	require.NoError(t, err)
	claims, err := op.Claims()
	require.NoError(t, err)
	require.Equal(t, accData.PublicKey, claims.SystemAccount)
}

func TestOperator_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Operator)
		isValid bool
	}{
		{
			name:    "valid operator",
			modify:  func(op *Operator) {},
			isValid: true,
		},
		{
			name: "invalid nkey",
			modify: func(op *Operator) {
				op.nk.Type = TypeAccount
			},
			isValid: false,
		},
		{
			name: "invalid signing key",
			modify: func(op *Operator) {
				op.sk.Type = TypeAccount
			},
			isValid: false,
		},
		{
			name: "invalid claims",
			modify: func(op *Operator) {
				op.JWT = "invalid"
			},
			isValid: false,
		},
		{
			name: "claims signing key mismatch",
			modify: func(op *Operator) {
				claims, err := jwt.DecodeOperatorClaims(op.JWT)
				require.NoError(t, err)
				claims.SigningKeys = jwt.StringList{}
				op.JWT, err = claims.Encode(op.SigningKey().KeyPair())
				require.NoError(t, err)
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := NewOperator(testutil.RandName(), id.New[NamespaceID]())
			require.NoError(t, err)

			tt.modify(op)

			err = op.Validate()
			if tt.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
