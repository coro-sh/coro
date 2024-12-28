package tkn

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	defaultTokenLength = 38
	alphanumericChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

type Option func(iss *Issuer)

// WithPepper adds the pepper (secret) to tokens before hashing.
func WithPepper(pepper []byte) Option {
	return func(iss *Issuer) {
		iss.pepper = pepper
	}
}

type Issuer struct {
	pepper []byte
}

func NewIssuer(opts ...Option) *Issuer {
	var iss Issuer
	for _, opt := range opts {
		opt(&iss)
	}
	return &iss
}

type GenerateOption func(opts *generateOptions)

// Config holds configuration parameters for token generation and hashing.
type generateOptions struct {
	length int    // Length of the random part of the token
	prefix string // Prefix to prepend to the token
}

// WithLength sets the length of the random part of the token.
func WithLength(length int) GenerateOption {
	return func(opts *generateOptions) {
		opts.length = length
	}
}

// WithPrefix sets the prefix to prepend to the token.
func WithPrefix(prefix string) GenerateOption {
	return func(opts *generateOptions) {
		opts.prefix = prefix
	}
}

// Generate generates a secure random token from the specified Config.
func (i *Issuer) Generate(opts ...GenerateOption) (string, error) {
	options := generateOptions{
		length: defaultTokenLength, // 226 bits of entropy
		prefix: "",
	}
	for _, opt := range opts {
		opt(&options)
	}

	length := options.length

	var sb strings.Builder
	sb.Grow(length)
	charSetLen := len(alphanumericChars)
	for i := 0; i < length; i++ {
		idx, err := randInt(charSetLen)
		if err != nil {
			return "", err
		}
		sb.WriteByte(alphanumericChars[idx])
	}

	return options.prefix + sb.String(), nil
}

// Hash hashes a token using Argon2id.
func (i *Issuer) Hash(token string) (string, error) {
	salt := make([]byte, 16) // 128-bit salt
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %v", err)
	}

	tokenBytes := []byte(token)
	if i.pepper != nil {
		tokenBytes = append(tokenBytes, i.pepper...)
	}

	hash := argon2.IDKey(tokenBytes, salt, 1, 64*1024, 4, 32)
	return encodeHash(salt, hash), nil
}

// Verify verifies a token against a hashed token.
func (i *Issuer) Verify(token string, hashedToken string) (bool, error) {
	salt, hash, err := decodeHash(hashedToken)
	if err != nil {
		return false, err
	}

	tokenBytes := []byte(token)
	if i.pepper != nil {
		tokenBytes = append(tokenBytes, i.pepper...)
	}

	newHash := argon2.IDKey(tokenBytes, salt, 1, 64*1024, 4, 32)
	return subtle.ConstantTimeCompare(newHash, hash) == 1, nil
}

func encodeHash(salt, hash []byte) string {
	return base64.RawStdEncoding.EncodeToString(salt) + ":" + base64.RawStdEncoding.EncodeToString(hash)
}

func decodeHash(encodedHash string) (salt, hash []byte, err error) {
	parts := strings.SplitN(encodedHash, ":", 2)
	if len(parts) != 2 {
		return nil, nil, errors.New("invalid hashed token format")
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode salt: %v", err)
	}
	hash, err = base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode hash: %v", err)
	}
	return salt, hash, nil
}

func randInt(limit int) (int, error) {
	if limit <= 0 {
		return 0, errors.New("max must be positive")
	}
	b := make([]byte, 1)
	_, err := rand.Read(b)
	if err != nil {
		return 0, err
	}
	return int(b[0]) % limit, nil
}
