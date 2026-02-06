package entity

import (
	"errors"
	"fmt"

	"github.com/joshjon/kit/id"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"go.jetify.com/typeid"
)

type operatorPrefix struct{}

func (operatorPrefix) Prefix() string { return "op" }

// OperatorID is the unique identifier for an Operator.
type OperatorID struct {
	typeid.TypeID[operatorPrefix]
}

// OperatorIdentity holds the static identifier fields of an Operator.
type OperatorIdentity struct {
	ID          OperatorID  `json:"id"`
	NamespaceID NamespaceID `json:"namespace_id"`
	JWT         string      `json:"jwt"`
}

// OperatorData represents the data of an Operator such as identity
// information.
type OperatorData struct {
	OperatorIdentity
	Name            string `json:"name"`
	PublicKey       string `json:"public_key"`
	LastConnectTime *int64 `json:"last_connect_time,omitempty"` // unix seconds
}

func (o *OperatorIdentity) GetNamespaceID() NamespaceID {
	return o.NamespaceID
}

// Validate validates the operator data matches the claims in the JWT.
func (o OperatorData) Validate() error {
	claims, err := jwt.DecodeOperatorClaims(o.JWT)
	if err != nil {
		return err
	}
	if claims.Name != o.Name {
		return errors.New("claims name mismatch")
	}
	if claims.Subject != o.PublicKey {
		return errors.New("name mismatch")
	}
	return nil
}

// Operator is responsible for running NATS servers and issuing account JWTs.
type Operator struct {
	OperatorIdentity
	nk              *Nkey
	sk              *Nkey
	lastConnectTime *int64
}

// NewOperator creates a new Operator.
func NewOperator(name string, namespaceID NamespaceID) (*Operator, error) {
	id := id.New[OperatorID]()

	nk, err := NewNkey(id.String(), TypeOperator, false)
	if err != nil {
		return nil, err
	}

	nkPub, err := nk.KeyPair().PublicKey()
	if err != nil {
		return nil, err
	}

	sk, err := NewNkey(id.String(), TypeOperator, true)
	if err != nil {
		return nil, err
	}

	skPub, err := sk.KeyPair().PublicKey()
	if err != nil {
		return nil, err
	}

	claims := jwt.NewOperatorClaims(nkPub)
	claims.Name = name
	claims.StrictSigningKeyUsage = true
	claims.SigningKeys.Add(skPub)

	opJWT, err := claims.Encode(nk.KeyPair())
	if err != nil {
		return nil, err
	}

	return &Operator{
		OperatorIdentity: OperatorIdentity{
			ID:          id,
			NamespaceID: namespaceID,
			JWT:         opJWT,
		},
		nk: nk,
		sk: sk,
	}, nil
}

// NewOperatorFromData creates a new Operator from existing nkeys and OperatorData.
func NewOperatorFromData(nk *Nkey, sk *Nkey, data OperatorData) (*Operator, error) {
	if err := data.Validate(); err != nil {
		return nil, err
	}
	op := &Operator{
		OperatorIdentity: data.OperatorIdentity,
		lastConnectTime:  data.LastConnectTime,
		nk:               nk,
		sk:               sk,
	}
	if err := op.Validate(); err != nil {
		return nil, err
	}
	return op, nil
}

// NKey returns the Operator's Nkey.
func (o *Operator) NKey() *Nkey {
	return o.nk
}

// SigningKey returns the Operator's signing key.
func (o *Operator) SigningKey() *Nkey {
	return o.sk
}

// Claims decodes and returns the Operator's JWT claims.
func (o *Operator) Claims() (*jwt.OperatorClaims, error) {
	return jwt.DecodeOperatorClaims(o.JWT)
}

// Data returns the Operator's data.
func (o *Operator) Data() (OperatorData, error) {
	claims, err := o.Claims()
	if err != nil {
		return OperatorData{}, err
	}
	return OperatorData{
		OperatorIdentity: o.OperatorIdentity,
		Name:             claims.Name,
		PublicKey:        claims.Subject,
		LastConnectTime:  o.lastConnectTime,
	}, nil
}

type UpdateOperatorParams struct {
	Name            string
	LastConnectTime *int64 // unix seconds
}

// Update updates the Operator and creates a new JWT.
func (o *Operator) Update(update UpdateOperatorParams) error {
	o.lastConnectTime = update.LastConnectTime
	return o.updateClaims(func(claims *jwt.OperatorClaims) {
		claims.Name = update.Name
	})
}

// SetSystemAccount sets the system account for the Operator.
func (o *Operator) SetSystemAccount(sysAcc *Account) error {
	sysAccData, err := sysAcc.Data()
	if err != nil {
		return err
	}
	return o.updateClaims(func(claims *jwt.OperatorClaims) {
		claims.SystemAccount = sysAccData.PublicKey
	})
}

// SetNewSystemAccountAndUser creates a new System Account and sets it on the
// operator. It also creates a new System User for the System Account.
func (o *Operator) SetNewSystemAccountAndUser() (*Account, *User, error) {
	sysAcc, err := NewSystemAccount(o)
	if err != nil {
		return nil, nil, err
	}

	if err = o.SetSystemAccount(sysAcc); err != nil {
		return nil, nil, err
	}

	sysUser, err := NewSystemUser(sysAcc)
	if err != nil {
		return nil, nil, err
	}

	return sysAcc, sysUser, nil
}

// Validate validates the Operator.
func (o *Operator) Validate() error {
	if err := validateNkey(o.nk, TypeOperator); err != nil {
		return fmt.Errorf("validate nkey: %w", err)
	}

	if err := validateNkey(o.sk, TypeOperator); err != nil {
		return fmt.Errorf("validate signing key: %w", err)
	}

	claims, err := o.Claims()
	if err != nil {
		return err
	}

	var vr jwt.ValidationResults
	claims.Validate(&vr)
	if len(vr.Errors()) > 0 {
		return fmt.Errorf("invalid claims: %w", errors.Join(vr.Errors()...))
	}

	nkPubKey, err := o.nk.KeyPair().PublicKey()
	if err != nil {
		return err
	}
	if !nkeys.IsValidPublicOperatorKey(claims.Subject) || claims.Subject != nkPubKey {
		return errors.New("invalid claims public operator key")
	}

	skPubKey, err := o.sk.KeyPair().PublicKey()
	if err != nil {
		return err
	}
	if !claims.SigningKeys.Contains(skPubKey) {
		return errors.New("claims signing key mismatch")
	}

	return nil
}

func (o *Operator) updateClaims(update func(claims *jwt.OperatorClaims)) error {
	claims, err := jwt.DecodeOperatorClaims(o.JWT)
	if err != nil {
		return err
	}
	update(claims)

	var vr jwt.ValidationResults
	claims.Validate(&vr)
	if len(vr.Errors()) > 0 {
		return fmt.Errorf("invalid claim update: %w", errors.Join(vr.Errors()...))
	}

	tkn, err := claims.Encode(o.nk.KeyPair())
	if err != nil {
		return err
	}

	o.JWT = tkn
	return nil
}

type OperatorNATSStatus struct {
	Connected   bool   `json:"connected"`
	ConnectTime *int64 `json:"connect_time"` // unix seconds
}

func validateNkey(key *Nkey, nkeyType Type) error {
	if err := key.Validate(); err != nil {
		return err
	}
	if key.Type != nkeyType {
		return errors.New("invalid entity type")
	}
	return nil
}
