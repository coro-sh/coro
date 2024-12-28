package tkn

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPrefix       = "prefix_"
	testToken        = "test_token"
	testInvalidToken = "invalid_token"
	testPepper       = "secret_pepper"
)

func TestIssuer_Generate(t *testing.T) {
	iss := NewIssuer()

	tests := []struct {
		name       string
		options    []GenerateOption
		wantLength int
		wantPrefix string
	}{
		{
			name:       "default generation",
			options:    nil,
			wantLength: defaultTokenLength,
			wantPrefix: "",
		},
		{
			name:       "with custom length",
			options:    []GenerateOption{WithLength(50)},
			wantLength: 50,
			wantPrefix: "",
		},
		{
			name:       "with prefix",
			options:    []GenerateOption{WithPrefix("prefix_")},
			wantLength: len("prefix_") + defaultTokenLength,
			wantPrefix: testPrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := iss.Generate(tt.options...)
			require.NoError(t, err)
			assert.Len(t, token, tt.wantLength)
			assert.True(t, strings.HasPrefix(token, tt.wantPrefix))
		})
	}
}

func TestIssuer_Hash(t *testing.T) {
	tests := []struct {
		name   string
		pepper []byte
	}{
		{
			name:   "hash token without pepper",
			pepper: nil,
		},
		{
			name:   "hash token with pepper",
			pepper: []byte(testPepper),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iss := NewIssuer()
			if tt.pepper != nil {
				iss = NewIssuer(WithPepper(tt.pepper))
			}

			token := testToken
			hashedToken, err := iss.Hash(token)
			require.NoError(t, err)
			assert.NotEmpty(t, hashedToken)
		})
	}
}

func TestIssuer_HashVerify(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		hashedToken string
		verifyToken string
		wantValid   bool
		pepper      []byte
	}{
		{
			name:        "valid token without pepper",
			token:       testToken,
			verifyToken: testToken,
			wantValid:   true,
			pepper:      nil,
		},
		{
			name:        "invalid token without pepper",
			token:       testToken,
			verifyToken: testInvalidToken,
			wantValid:   false,
			pepper:      nil,
		},
		{
			name:        "valid token with pepper",
			token:       testToken,
			verifyToken: testToken,
			wantValid:   true,
			pepper:      []byte(testPepper),
		},
		{
			name:        "invalid token with pepper",
			token:       testToken,
			verifyToken: testInvalidToken,
			wantValid:   false,
			pepper:      []byte(testPepper),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iss := NewIssuer()
			if tt.pepper != nil {
				iss = NewIssuer(WithPepper(tt.pepper))
			}

			hashedToken, err := iss.Hash(tt.token)
			require.NoError(t, err)

			valid, err := iss.Verify(tt.verifyToken, hashedToken)
			require.NoError(t, err)
			assert.Equal(t, tt.wantValid, valid)
		})
	}
}
