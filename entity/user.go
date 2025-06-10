package entity

import (
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"go.jetify.com/typeid"

	"github.com/coro-sh/coro/constants"
)

type userPrefix struct{}

func (userPrefix) Prefix() string { return "usr" }

// UserID is the unique identifier for a User.
type UserID struct {
	typeid.TypeID[userPrefix]
}

// UserIdentity holds the static identifier fields of a User.
type UserIdentity struct {
	ID          UserID      `json:"id"`
	NamespaceID NamespaceID `json:"namespace_id"`
	OperatorID  OperatorID  `json:"operator_id"`
	AccountID   AccountID   `json:"account_id"`
}

func (u *UserIdentity) GetNamespaceID() NamespaceID {
	return u.NamespaceID
}

// UserData represents the data of a User such as identity information and
// configuration.
type UserData struct {
	UserIdentity
	Name        string         `json:"name"`
	JWT         string         `json:"jwt"`
	JWTDuration *time.Duration `json:"-"`
}

// Validate checks if the user data matches the claims in the JWT.
func (u UserData) Validate() error {
	claims, err := jwt.DecodeUserClaims(u.JWT)
	if err != nil {
		return err
	}
	if claims.Name != u.Name {
		return errors.New("claims name mismatch")
	}
	return nil
}

// User is issued by an account and provides authorization to an account's
// subject space.
type User struct {
	UserIdentity
	nk          *Nkey
	jwt         string
	jwtDuration *time.Duration
}

// NewUser creates a new User for the specified Account.
func NewUser(name string, account *Account) (*User, error) {
	return newUser(name, account)
}

// NewUserFromData creates a new User from an existing nkey and UserData.
func NewUserFromData(nk *Nkey, data UserData) (*User, error) {
	if err := data.Validate(); err != nil {
		return nil, err
	}
	if _, err := jwt.DecodeUserClaims(data.JWT); err != nil {
		return nil, err
	}
	user := &User{
		UserIdentity: data.UserIdentity,
		nk:           nk,
		jwt:          data.JWT,
		jwtDuration:  data.JWTDuration,
	}
	if err := user.Validate(); err != nil {
		return nil, err
	}
	return user, nil
}

// NewSystemUser creates a System User for a System Account.
func NewSystemUser(sysAcc *Account) (*User, error) {
	isSysAcc, err := sysAcc.IsSystemAccount()
	if err != nil {
		return nil, err
	}
	if !isSysAcc {
		return nil, errors.New("not a system account")
	}
	return newUser("sys", sysAcc)
}

// NKey returns the User's nkey.
func (u *User) NKey() *Nkey {
	return u.nk
}

// Claims decodes and returns the User's JWT claims.
func (u *User) Claims() (*jwt.UserClaims, error) {
	return jwt.DecodeUserClaims(u.jwt)
}

// Data returns the UserData representation of the User.
func (u *User) Data() (UserData, error) {
	claims, err := u.Claims()
	if err != nil {
		return UserData{}, err
	}
	return UserData{
		UserIdentity: u.UserIdentity,
		Name:         claims.Name,
		JWT:          u.jwt,
		JWTDuration:  u.jwtDuration,
	}, nil
}

// JWT returns the JWT for the User.
func (u *User) JWT() string {
	return u.jwt
}

// RefreshJWT updates the JWT for the User using the specified Account.
func (u *User) RefreshJWT(account *Account) (string, *jwt.UserClaims, error) {
	if u.AccountID != account.ID {
		return "", nil, errors.New("user jwt account signer conflict")
	}
	claims, err := jwt.DecodeUserClaims(u.jwt)
	if err != nil {
		return "", nil, err
	}

	accData, err := account.Data()
	if err != nil {
		return "", nil, err
	}

	if accData.UserJWTDuration != nil {
		claims.Expires = time.Now().Add(*accData.UserJWTDuration).Unix()
	} else if u.jwtDuration != nil {
		claims.Expires = time.Now().Add(*u.jwtDuration).Unix()
	}

	u.jwt, err = claims.Encode(account.SigningKey().KeyPair())
	if err != nil {
		return "", nil, err
	}

	return u.jwt, claims, nil
}

// IsSystemUser checks if the User is a system User.
func (u *User) IsSystemUser() (bool, error) {
	data, err := u.Data()
	if err != nil {
		return false, err
	}
	return data.Name == constants.SysUserName, nil
}

// Validate checks the validity of the User's claims and cryptographic keys.
func (u *User) Validate() error {
	claims, err := u.Claims()
	if err != nil {
		return err
	}

	var vr jwt.ValidationResults
	claims.Validate(&vr)
	if len(vr.Errors()) > 0 {
		return fmt.Errorf("invalid claims: %w", errors.Join(vr.Errors()...))
	}

	if !nkeys.IsValidPublicUserKey(claims.Subject) {
		return errors.New("invalid claims public user key")
	}

	return nil
}

// UpdateUserParams contains parameters for updating a User.
type UpdateUserParams struct {
	Name          string
	Subscriptions int64
	PayloadSize   int64
	JWTDuration   *time.Duration
}

// Update modifies the User's claims and updates the JWT.
func (u *User) Update(account *Account, update UpdateUserParams) error {
	accData, err := account.Data()
	if err != nil {
		return err
	}

	claims, err := u.Claims()
	if err != nil {
		return err
	}

	claims.Name = update.Name
	claims.Subs = update.Subscriptions
	claims.Limits.Payload = update.PayloadSize

	if accData.UserJWTDuration != nil {
		claims.Expires = time.Now().Add(*accData.UserJWTDuration).Unix()
	} else if update.JWTDuration != nil {
		claims.Expires = time.Now().Add(*update.JWTDuration).Unix()
	}

	tkn, err := claims.Encode(account.SigningKey().KeyPair())
	if err != nil {
		return err
	}

	u.jwt = tkn
	u.jwtDuration = update.JWTDuration
	return nil
}

func newUser(name string, account *Account) (*User, error) {
	if account.SigningKey().Type != TypeAccount {
		return nil, errors.New("signing key must be from an account")
	}

	accPubKey, err := account.NKey().KeyPair().PublicKey()
	if err != nil {
		return nil, err
	}

	id := NewID[UserID]()

	nk, err := NewNkey(id.String(), TypeUser, false)
	if err != nil {
		return nil, err
	}

	nkPub, err := nk.KeyPair().PublicKey()
	if err != nil {
		return nil, err
	}

	claims := jwt.NewUserClaims(nkPub)
	claims.Name = name
	claims.IssuerAccount = accPubKey

	userJWT, err := claims.Encode(account.SigningKey().KeyPair())
	if err != nil {
		return nil, err
	}

	return &User{
		UserIdentity: UserIdentity{
			ID:          id,
			NamespaceID: account.NamespaceID,
			OperatorID:  account.OperatorID,
			AccountID:   account.ID,
		},
		nk:  nk,
		jwt: userJWT,
	}, nil
}
