package encrypt

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

var (
	errorAESKeyLength        = errors.New("invalid key length (must be 16, 24, or 32 bytes)")
	errorAESCiphertextLength = errors.New("invalid ciphertext length")
)

var _ Encrypter = (*AES)(nil)

// AES provides encryption and decryption using the AES-GCM mode.
type AES struct {
	key []byte
}

// NewAES creates a new AES instance with the provided key.
// The key must be one of the following lengths: 16, 24, or 32 bytes,
// corresponding to AES-128, AES-192, and AES-256, respectively.
func NewAES(key string) (*AES, error) {
	switch len(key) {
	default:
		return nil, errorAESKeyLength
	case 16, 24, 32:
		break
	}
	return &AES{
		key: []byte(key),
	}, nil
}

// Encrypt encrypts the given plaintext using AES-GCM.
// The context parameter is ignored but is included for interface compliance.
// The resulting ciphertext includes the nonce prepended to the encrypted data.
func (a *AES) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts the given ciphertext using AES-GCM.
// The context parameter is ignored but is included for interface compliance.
// The ciphertext must include the nonce as its prefix.
func (a *AES) Decrypt(_ context.Context, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errorAESCiphertextLength
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
