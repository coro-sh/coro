package entity

import (
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"go.jetify.com/typeid"

	"github.com/coro-sh/coro/internal/constants"
)

type accountPrefix struct{}

func (accountPrefix) Prefix() string { return "acc" }

// AccountID is the unique identifier for an Account.
type AccountID struct {
	typeid.TypeID[accountPrefix]
}

// AccountIdentity holds the static identifier fields of an Account.
type AccountIdentity struct {
	ID          AccountID   `json:"id"`
	NamespaceID NamespaceID `json:"namespace_id"`
	OperatorID  OperatorID  `json:"operator_id"`
	JWT         string      `json:"jwt"`
}

func (a *AccountIdentity) GetNamespaceID() NamespaceID {
	return a.NamespaceID
}

// AccountData represents the data of an account such as identity information
// and configuration.
type AccountData struct {
	AccountIdentity
	Name            string         `json:"name"`
	PublicKey       string         `json:"public_key"`
	UserJWTDuration *time.Duration `json:"-"`
}

// Validate validates the account data matches the claims in the JWT.
func (a AccountData) Validate() error {
	claims, err := jwt.DecodeAccountClaims(a.JWT)
	if err != nil {
		return err
	}
	if claims.Name != a.Name {
		return errors.New("claims name mismatch")
	}
	if claims.Subject != a.PublicKey {
		return errors.New("name mismatch")
	}
	return nil
}

// Account is responsible for issuing user JWTs
type Account struct {
	AccountIdentity
	nk              *Nkey
	sk              *Nkey
	userJWTDuration *time.Duration
}

// NewAccount creates a new Account
func NewAccount(name string, operator *Operator) (*Account, error) {
	return newAccount(name, operator)
}

// NewAccountFromData creates a new Account from existing nkeys and AccountData.
func NewAccountFromData(nk *Nkey, sk *Nkey, data AccountData) (*Account, error) {
	if err := data.Validate(); err != nil {
		return nil, err
	}
	if _, err := jwt.DecodeAccountClaims(data.JWT); err != nil {
		return nil, err
	}
	acc := &Account{
		AccountIdentity: data.AccountIdentity,
		nk:              nk,
		sk:              sk,
		userJWTDuration: data.UserJWTDuration,
	}
	if err := acc.Validate(); err != nil {
		return nil, err
	}
	return acc, nil
}

// NewSystemAccount creates a new System Account.
func NewSystemAccount(operator *Operator) (*Account, error) {
	acc, err := newAccount(constants.SysAccountName, operator, func(claims *jwt.AccountClaims) {
		claims.Exports = jwt.Exports{
			&jwt.Export{
				Name:                 "account-monitoring-services",
				Subject:              "$SYS.REQ.ACCOUNT.*.*",
				Type:                 jwt.Service,
				ResponseType:         jwt.ResponseTypeStream,
				AccountTokenPosition: 4,
				Info: jwt.Info{
					Description: `Request account specific monitoring services for: SUBSZ, CONNZ, LEAFZ, JSZ and INFO`,
					InfoURL:     "https://docs.nats.io/nats-server/configuration/sys_accounts",
				},
			}, &jwt.Export{
				Name:                 "account-monitoring-streams",
				Subject:              "$SYS.ACCOUNT.*.>",
				Type:                 jwt.Stream,
				AccountTokenPosition: 3,
				Info: jwt.Info{
					Description: `Account specific monitoring stream`,
					InfoURL:     "https://docs.nats.io/nats-server/configuration/sys_accounts",
				},
			},
		}
	})
	if err != nil {
		return nil, err
	}

	return acc, err
}

// NKey returns the account's nkey.
func (a *Account) NKey() *Nkey {
	return a.nk
}

// SigningKey returns the account's signing key.
func (a *Account) SigningKey() *Nkey {
	return a.sk
}

// Claims decodes and returns the Account's JWT claims.
func (a *Account) Claims() (*jwt.AccountClaims, error) {
	return jwt.DecodeAccountClaims(a.JWT)
}

// Data returns the account's data.
func (a *Account) Data() (AccountData, error) {
	claims, err := a.Claims()
	if err != nil {
		return AccountData{}, err
	}
	return AccountData{
		AccountIdentity: a.AccountIdentity,
		Name:            claims.Name,
		PublicKey:       claims.Subject,
		UserJWTDuration: a.userJWTDuration,
	}, nil
}

// IsSystemAccount returns true if the account is a system account.
func (a *Account) IsSystemAccount() (bool, error) {
	data, err := a.Data()
	if err != nil {
		return false, err
	}
	return data.Name == constants.SysAccountName, nil
}

// Validate validates the account.
func (a *Account) Validate() error {
	if err := validateNkey(a.nk, TypeAccount); err != nil {
		return fmt.Errorf("validate nkey: %w", err)
	}

	if err := validateNkey(a.sk, TypeAccount); err != nil {
		return fmt.Errorf("validate signing key: %w", err)
	}

	claims, err := a.Claims()
	if err != nil {
		return err
	}

	var vr jwt.ValidationResults
	claims.Validate(&vr)
	if len(vr.Errors()) > 0 {
		return fmt.Errorf("invalid claims: %w", errors.Join(vr.Errors()...))
	}

	if !nkeys.IsValidPublicAccountKey(claims.Subject) {
		return errors.New("invalid claims public account key")
	}

	skPubKey, err := a.sk.KeyPair().PublicKey()
	if err != nil {
		return err
	}
	if !claims.SigningKeys.Contains(skPubKey) {
		return errors.New("claims signing key mismatch")
	}

	return nil
}

type UpdateAccountParams struct {
	Name            string
	Subscriptions   int64
	PayloadSize     int64
	Imports         int64
	Exports         int64
	Connections     int64
	UserJWTDuration *time.Duration
}

// Update updates the account and creates a new JWT.
func (a *Account) Update(operator *Operator, update UpdateAccountParams) error {
	claims, err := a.Claims()
	if err != nil {
		return err
	}

	claims.Name = update.Name
	claims.Limits.Subs = update.Subscriptions
	claims.Limits.Payload = update.PayloadSize
	claims.Limits.Imports = update.Imports
	claims.Limits.Exports = update.Exports
	claims.Limits.Conn = update.Connections

	tkn, err := claims.Encode(operator.SigningKey().KeyPair())
	if err != nil {
		return err
	}

	a.JWT = tkn
	a.userJWTDuration = update.UserJWTDuration
	return nil
}

func newAccount(name string, operator *Operator, claimsModifiers ...func(claims *jwt.AccountClaims)) (*Account, error) {
	if operator.SigningKey().Type != TypeOperator {
		return nil, errors.New("signing key must be from an operator")
	}

	id := NewID[AccountID]()

	nk, err := NewNkey(id.String(), TypeAccount, false)
	if err != nil {
		return nil, err
	}

	nkPub, err := nk.KeyPair().PublicKey()
	if err != nil {
		return nil, err
	}

	sk, err := NewNkey(id.String(), TypeAccount, true)
	if err != nil {
		return nil, err
	}

	skPub, err := sk.KeyPair().PublicKey()
	if err != nil {
		return nil, err
	}

	claims := jwt.NewAccountClaims(nkPub)
	claims.Name = name
	claims.SigningKeys.Add(skPub)

	for _, mod := range claimsModifiers {
		mod(claims)
	}

	accJWT, err := claims.Encode(operator.SigningKey().KeyPair())
	if err != nil {
		return nil, err
	}

	return &Account{
		AccountIdentity: AccountIdentity{
			ID:          id,
			NamespaceID: operator.NamespaceID,
			OperatorID:  operator.ID,
			JWT:         accJWT,
		},
		nk: nk,
		sk: sk,
	}, nil
}
