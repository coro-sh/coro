package entity

import (
	"fmt"
	"math"
	"time"

	"github.com/cohesivestack/valgo"
	"github.com/nats-io/nkeys"
	"go.jetify.com/typeid"

	"github.com/coro-sh/coro/constants"
)

const (
	maxNameLength = 100
	maxLimit      = math.MaxInt
	maxSeconds    = int64(time.Hour * time.Duration(290*365*24)) // 290 years
	noLimit       = -1
)

type CreateNamespaceRequest struct {
	Name string `json:"name"`
}

func (r CreateNamespaceRequest) Validate() error {
	return valgo.Is(namespaceNameValidator(r.Name, "name")).Error()
}

type DeleteNamespaceRequest struct {
	ID string `param:"namespace_id" json:"-"`
}

func (r DeleteNamespaceRequest) Validate() error {
	return valgo.In("params", valgo.Is(IDValidator[NamespaceID](r.ID, "namespace_id"))).Error()
}

type NamespaceResponse struct {
	Namespace
}

type CreateOperatorRequest struct {
	NamespaceID string `param:"namespace_id" json:"-"`
	Name        string `json:"name"`
}

func (r CreateOperatorRequest) Validate() error {
	v := valgo.In("params", valgo.Is(IDValidator[NamespaceID](r.NamespaceID, "namespace_id")))
	return v.Is(operatorNameValidator(r.Name, "name")).Error()
}

type UpdateOperatorRequest struct {
	ID   string `param:"operator_id" json:"-"`
	Name string `json:"name"`
}

func (r UpdateOperatorRequest) Validate() error {
	v := valgo.In("params", valgo.Is(IDValidator[OperatorID](r.ID, "operator_id")))
	v.Is(operatorNameValidator(r.Name, "name"))
	return v.Error()
}

type GetOperatorRequest struct {
	ID string `param:"operator_id" json:"-"`
}

func (r GetOperatorRequest) Validate() error {
	return valgo.In("params", valgo.Is(IDValidator[OperatorID](r.ID, "operator_id"))).Error()
}

type GetOperatorJWTRequest struct {
	PK string `param:"operator_public_key" json:"-"`
}

func (r GetOperatorJWTRequest) Validate() error {
	return valgo.In("params", valgo.Is(
		valgo.String(r.PK, "operator_public_key").Not().Blank().Passing(func(_ string) bool {
			return nkeys.IsValidPublicOperatorKey(r.PK)
		}, "Must be an operator public key"),
	)).Error()
}

type ListOperatorsRequest struct {
	NamespaceID string `param:"namespace_id" json:"-"`
}

func (r ListOperatorsRequest) Validate() error {
	return valgo.In("params", valgo.Is(IDValidator[NamespaceID](r.NamespaceID, "namespace_id"))).Error()
}

type OperatorResponse struct {
	OperatorData
	Status OperatorNATSStatus `json:"status"`
}

type AccountLimits struct {
	Subscriptions       *int64 `json:"subscriptions"`
	PayloadSize         *int64 `json:"payload_size"`
	Imports             *int64 `json:"imports"`
	Exports             *int64 `json:"exports"`
	Connections         *int64 `json:"connections"`
	UserJWTDurationSecs *int64 `json:"user_jwt_duration_secs"`
}

func (a AccountLimits) validation() *valgo.Validation {
	return valgo.Is(
		valgo.Int64P(a.Subscriptions, "subscriptions").Nil().Or().Between(0, maxLimit),
		valgo.Int64P(a.PayloadSize, "payload_size").Nil().Or().Between(0, maxLimit),
		valgo.Int64P(a.Imports, "imports").Nil().Or().Between(0, maxLimit),
		valgo.Int64P(a.Exports, "exports").Nil().Or().Between(0, maxLimit),
		valgo.Int64P(a.Connections, "connections").Nil().Or().Between(0, maxLimit),
		valgo.Int64P(a.UserJWTDurationSecs, "user_jwt_duration_secs").Nil().Or().Between(0, maxSeconds),
	)
}

type CreateAccountRequest struct {
	OperatorID string         `param:"operator_id" json:"-"`
	Name       string         `json:"name"`
	Limits     *AccountLimits `json:"limits"`
}

func (r CreateAccountRequest) Validate() error {
	v := valgo.Is(accountNameValidator(r.Name, "name"))
	if r.Limits != nil {
		v.In("limits", r.Limits.validation())
	}
	v.In("params", valgo.Is(IDValidator[OperatorID](r.OperatorID, "operator_id")))
	return v.Error()
}

type GetAccountRequest struct {
	ID string `param:"account_id" json:"-"`
}

func (r GetAccountRequest) Validate() error {
	return valgo.In("params", valgo.Is(IDValidator[AccountID](r.ID, "account_id"))).Error()
}

type GetAccountJWTRequest struct {
	PK string `param:"account_public_key"  json:"-"`
}

func (r GetAccountJWTRequest) Validate() error {
	return valgo.In("params", valgo.Is(
		valgo.String(r.PK, "account_public_key").Not().Blank().Passing(func(_ string) bool {
			return nkeys.IsValidPublicAccountKey(r.PK)
		}, "Must be an account public key"),
	)).Error()
}

type UpdateAccountRequest struct {
	ID     string         `param:"account_id" json:"-"`
	Name   string         `json:"name"`
	Limits *AccountLimits `json:"limits"`
}

func (r UpdateAccountRequest) Validate() error {
	v := valgo.In("params", valgo.Is(IDValidator[AccountID](r.ID, "account_id")))
	v.Is(accountNameValidator(r.Name, "name"))
	if r.Limits != nil {
		v.In("limits", r.Limits.validation())
	}
	return v.Error()
}

type ListAccountsRequest struct {
	OperatorID string `param:"operator_id" json:"-"`
}

func (r ListAccountsRequest) Validate() error {
	return valgo.In("params", valgo.Is(IDValidator[OperatorID](r.OperatorID, "operator_id"))).Error()
}

type AccountResponse struct {
	AccountData
	Limits AccountLimits `json:"limits"`
}

type UserLimits struct {
	Subscriptions   *int64 `json:"subscriptions"`
	PayloadSize     *int64 `json:"payload_size"`
	JWTDurationSecs *int64 `json:"jwt_duration_secs"`
}

func (u UserLimits) validation() *valgo.Validation {
	return valgo.Is(
		valgo.Int64P(u.Subscriptions, "subscriptions").Nil().Or().Between(0, maxLimit),
		valgo.Int64P(u.PayloadSize, "payload_size").Nil().Or().Between(0, maxLimit),
		valgo.Int64P(u.JWTDurationSecs, "jwt_duration_secs").Nil().Or().Between(0, maxSeconds),
	)
}

type CreateUserRequest struct {
	AccountID string      `param:"account_id" json:"-"`
	Name      string      `json:"name"`
	Limits    *UserLimits `json:"limits"`
}

func (r CreateUserRequest) Validate() error {
	v := valgo.Is(userNameValidator(r.Name, "name"))
	v.In("params", valgo.Is(IDValidator[AccountID](r.AccountID, "account_id")))
	return v.Error()
}

type UpdateUserRequest struct {
	ID     string      `param:"user_id" json:"-"`
	Name   string      `json:"name"`
	Limits *UserLimits `json:"limits"`
}

func (r UpdateUserRequest) Validate() error {
	v := valgo.In("params", valgo.Is(IDValidator[UserID](r.ID, "user_id")))
	v.Is(userNameValidator(r.Name, "name"))
	if r.Limits != nil {
		v.In("limits", r.Limits.validation())
	}
	return v.Error()
}

type GetUserRequest struct {
	ID string `param:"user_id" json:"-"`
}

func (r GetUserRequest) Validate() error {
	return valgo.In("params", valgo.Is(IDValidator[UserID](r.ID, "user_id"))).Error()
}

type ListUsersRequest struct {
	AccountID string `param:"account_id" json:"-"`
}

func (r ListUsersRequest) Validate() error {
	return valgo.In("params", valgo.Is(IDValidator[AccountID](r.AccountID, "account_id"))).Error()
}

type UserResponse struct {
	UserData
	PublicKey string     `json:"public_key"`
	Limits    UserLimits `json:"limits"`
}

type UserJWTIssuanceResponse struct {
	UserJWTIssuance
	Active bool `json:"active"`
}

type GenerateProxyTokenRequest struct {
	OperatorID string `param:"operator_id" json:"-"`
}

func (r GenerateProxyTokenRequest) Validate() error {
	return valgo.In("params", valgo.Is(IDValidator[OperatorID](r.OperatorID, "operator_id"))).Error()
}

type GenerateProxyTokenResponse struct {
	Token string `json:"token"`
}

type GetProxyStatusRequest struct {
	OperatorID string `param:"operator_id" json:"-"`
}

func (r GetProxyStatusRequest) Validate() error {
	return valgo.In("params", valgo.Is(IDValidator[OperatorID](r.OperatorID, "operator_id"))).Error()
}

type GetProxyStatusResponse struct {
	Connected bool `json:"connected"`
}

func IDValidator[T ID, PT typeid.SubtypePtr[T]](id string, nameAndTitle ...string) valgo.Validator {
	entityName := GetTypeNameFromID[T]()

	var parsed T
	var parseErr error

	return valgo.String(id, nameAndTitle...).
		Passing(func(_ string) bool {
			parsed, parseErr = ParseID[T, PT](id)
			return parseErr == nil
		}, fmt.Sprintf("Must be a valid %s ID", entityName)).
		Passing(func(_ string) bool {
			if parseErr == nil {
				return parsed.Suffix() != "" || !parsed.IsZero()
			}
			return true
		}, "Must not be empty")
}

func nameValidator(name string, nameAndTitle ...string) *valgo.ValidatorString[string] {
	return valgo.String(name, nameAndTitle...).Not().Blank().MaxLength(maxNameLength)
}

func namespaceNameValidator(accName string, nameAndTitle ...string) valgo.Validator {
	return nameValidator(accName, nameAndTitle...).
		Passing(func(_ string) bool {
			return !constants.IsReservedNamespaceName(accName)
		}, "Name is a reserved value")
}

func operatorNameValidator(accName string, nameAndTitle ...string) valgo.Validator {
	return nameValidator(accName, nameAndTitle...).
		Passing(func(_ string) bool {
			return !constants.IsReservedOperatorName(accName)
		}, "Name is a reserved value")
}

func accountNameValidator(accName string, nameAndTitle ...string) valgo.Validator {
	return nameValidator(accName, nameAndTitle...).
		Passing(func(_ string) bool {
			return !constants.IsReservedAccountName(accName)
		}, "Name is a reserved value")
}

func userNameValidator(accName string, nameAndTitle ...string) valgo.Validator {
	return nameValidator(accName, nameAndTitle...).
		Passing(func(_ string) bool {
			return !constants.IsReservedUserName(accName)
		}, "Name is a reserved value")
}
