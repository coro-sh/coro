package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNkeyType_String(t *testing.T) {
	tests := []struct {
		name     string
		nkeyType Type
		expected string
	}{
		{
			name:     "Unspecified",
			nkeyType: TypeUnspecified,
			expected: "unspecified",
		},
		{
			name:     "Operator",
			nkeyType: TypeOperator,
			expected: "operator",
		},
		{
			name:     "Account",
			nkeyType: TypeAccount,
			expected: "account",
		},
		{
			name:     "User",
			nkeyType: TypeUser,
			expected: "user",
		},
		{
			name:     "Invalid",
			nkeyType: Type(999),
			expected: "unspecified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.nkeyType.String())
		})
	}
}

func TestNewNkey(t *testing.T) {
	tests := []struct {
		name      string
		nkeyType  Type
		expectErr bool
	}{
		{
			name:     "valid operator",
			nkeyType: TypeOperator,
		},
		{
			name:     "valid account",
			nkeyType: TypeAccount,
		},
		{
			name:     "valid user",
			nkeyType: TypeUser,
		},
		{
			name:      "invalid entity type",
			nkeyType:  TypeUnspecified,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testNewNkey := func(signingKey bool) {
				id := "nkey1"
				got, err := NewNkey(id, tt.nkeyType, signingKey)
				if tt.expectErr {
					assert.Error(t, err)
					assert.Nil(t, got)
				} else {
					require.NoError(t, err)
					require.NotNil(t, got)
					assert.Equal(t, id, got.ID)
					assert.Equal(t, tt.nkeyType, got.Type)
					assert.Equal(t, signingKey, got.SigningKey)
					assert.NotNil(t, got.KeyPair())

					kp := got.KeyPair()
					assert.NotNil(t, kp)
					seed, err := kp.Seed()
					assert.NoError(t, err)
					assert.Equal(t, got.Seed, seed)
				}
			}

			testNewNkey(false)
			testNewNkey(true)
		})
	}
}

func TestNewNkeyFromData(t *testing.T) {
	want, err := NewNkey("nkey1", TypeOperator, false)
	require.NoError(t, err)

	got, err := NewNkeyFromData(want.NkeyData)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
