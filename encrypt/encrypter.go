package encrypt

import "context"

// Encrypter defines an interface for encryption and decryption mechanisms.
// Implementations of this interface must support securely encrypting plaintext
// and decrypting ciphertext, optionally considering context for cancellation or
// deadlines.
type Encrypter interface {
	// Encrypt returns the encrypted ciphertext.
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)
	// Decrypt returns the decrypted plaintext.
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}
