package encrypt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/testutil"
)

func TestNewAES(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name: "valid AES-128 key",
			key:  "1234567890123456", // 16 bytes
		},
		{
			name: "valid AES-192 key",
			key:  "123456789012345678901234", // 24 bytes
		},
		{
			name: "valid AES-256 key",
			key:  "12345678901234567890123456789012", // 32 bytes
		},
		{
			name:    "key length too short",
			key:     "short_key", // < 16 bytes
			wantErr: true,
		},
		{
			name:    "key length too long",
			key:     "short_key", // > 32 bytes
			wantErr: true,
		},
		{
			name:    "Empty Key",
			key:     "", // Empty key
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAES(tt.key)
			if tt.wantErr {
				assert.ErrorIs(t, err, errorAESKeyLength)
				assert.Nil(t, got)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

func TestAES_EncryptDecrypt(t *testing.T) {
	aes, err := NewAES("1234567890123456")
	require.NoError(t, err)

	plaintext := []byte(testutil.RandString(100))

	ciphertext, err := aes.Encrypt(context.Background(), plaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	decrypted, err := aes.Decrypt(context.Background(), ciphertext)
	require.NoError(t, err)

	assert.Equal(t, plaintext, decrypted)
}

func TestAES_DecryptInvalidCiphertext(t *testing.T) {
	tests := []struct {
		name       string
		ciphertext []byte
		wantErr    string
	}{
		{
			name:       "Empty Ciphertext",
			ciphertext: []byte{},
			wantErr:    errorAESCiphertextLength.Error(),
		},
		{
			name:       "Short Ciphertext (no nonce)",
			ciphertext: []byte{1, 2, 3, 4},
			wantErr:    errorAESCiphertextLength.Error(),
		},
		{
			name:       "Tampered Ciphertext",
			ciphertext: append(make([]byte, 12), []byte("tampered data")...),
			wantErr:    "cipher: message authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aesObj, err := NewAES("1234567890123456")
			require.NoError(t, err)

			_, err = aesObj.Decrypt(context.Background(), tt.ciphertext)
			assert.Error(t, err, tt.wantErr)
		})
	}
}
