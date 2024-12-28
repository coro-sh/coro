package entity

import (
	"errors"
	"fmt"

	"github.com/nats-io/nkeys"
)

type NkeyData struct {
	ID         string // Unique identifier (entity ID).
	Seed       []byte // The seed used to generate the key pair.
	SigningKey bool   // Indicates if the key is a signing key.
	Type       Type   // The type of the key.
}

// Nkey represents a NATS key pair.
type Nkey struct {
	NkeyData
	kp nkeys.KeyPair // The nkeys.KeyPair associated with the Nkey.
}

// NewNkey creates a new Nkey by generating the appropriate key pair for the type.
func NewNkey(id string, nkeyType Type, signingKey bool) (*Nkey, error) {
	var kp nkeys.KeyPair
	var err error

	switch nkeyType {
	case TypeOperator:
		kp, err = nkeys.CreateOperator()
	case TypeAccount:
		kp, err = nkeys.CreateAccount()
	case TypeUser:
		kp, err = nkeys.CreateUser()
	case TypeUnspecified, typeEnd:
		fallthrough
	default:
		err = errors.New("invalid nkey type")
	}

	if err != nil {
		return nil, err
	}

	seed, err := kp.Seed()
	if err != nil {
		return nil, err
	}

	nkeyPair, err := nkeys.FromSeed(seed)
	if err != nil {
		return nil, err
	}

	return &Nkey{
		NkeyData: NkeyData{
			ID:         id,
			Type:       nkeyType,
			Seed:       seed,
			SigningKey: signingKey,
		},
		kp: nkeyPair,
	}, nil
}

// NewNkeyFromData creates a new Nkey from existing NkeyData.
func NewNkeyFromData(data NkeyData) (*Nkey, error) {
	nkeyPair, err := nkeys.FromSeed(data.Seed)
	if err != nil {
		return nil, err
	}
	return &Nkey{
		NkeyData: data,
		kp:       nkeyPair,
	}, nil
}

// KeyPair returns the nkeys.KeyPair associated with the Nkey.
func (n *Nkey) KeyPair() nkeys.KeyPair {
	return n.kp
}

// Validate checks if the Nkey is valid.
func (n *Nkey) Validate() error {
	_, err := nkeys.FromSeed(n.Seed)
	if err != nil {
		return fmt.Errorf("invalid seed: %w", err)
	}

	kpSeed, err := n.KeyPair().Seed()
	if err != nil {
		return fmt.Errorf("failed to get seed from key pair: %w", err)
	}

	if string(n.Seed) != string(kpSeed) {
		return fmt.Errorf("seed mismatch")
	}

	if n.Type <= TypeUnspecified || n.Type >= typeEnd {
		return errors.New("invalid nkey type")
	}

	return nil
}
