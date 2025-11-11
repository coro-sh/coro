package entity

import (
	"context"

	"github.com/coro-sh/coro/paginate"
	"github.com/coro-sh/coro/tx"
)

// Repository is the interface for performing CRUD operations on Namespaces,
// Operators, Accounts, Users, and Nkeys.
//
// Implementations must pass all tests in the RepositoryTestSuite to be
// considered compliant for use in the application.
type Repository interface {
	NamespaceRepository
	OperatorRepository
	AccountRepository
	UserRepository
	NkeyRepository
	tx.Repository[Repository]
}

type NamespaceRepository interface {
	CreateNamespace(ctx context.Context, namespace *Namespace) error
	ReadNamespace(ctx context.Context, id NamespaceID) (*Namespace, error)
	ReadNamespaceByName(ctx context.Context, name string, owner string) (*Namespace, error)
	BatchReadNamespaces(ctx context.Context, ids []NamespaceID) ([]*Namespace, error)
	ListNamespaces(ctx context.Context, owner string, filter paginate.PageFilter[NamespaceID]) ([]*Namespace, error)
	DeleteNamespace(ctx context.Context, id NamespaceID) error // must cascade delete operators, accounts, users, and nkeys
	CountOwnerNamespaces(ctx context.Context, owner string) (int64, error)
}

type OperatorRepository interface {
	CreateOperator(ctx context.Context, operator OperatorData) error
	UpdateOperator(ctx context.Context, operator OperatorData) error
	ReadOperator(ctx context.Context, id OperatorID) (OperatorData, error)
	ReadOperatorByName(ctx context.Context, name string) (OperatorData, error)
	ReadOperatorByPublicKey(ctx context.Context, pubKey string) (OperatorData, error)
	ListOperators(ctx context.Context, namespaceID NamespaceID, filter paginate.PageFilter[OperatorID]) ([]OperatorData, error)
	DeleteOperator(ctx context.Context, id OperatorID) error // must cascade delete accounts, users, and nkeys
	CountOwnerOperators(ctx context.Context, owner string) (int64, error)
}

type AccountRepository interface {
	CreateAccount(ctx context.Context, account AccountData) error
	UpdateAccount(ctx context.Context, account AccountData) error
	ReadAccount(ctx context.Context, id AccountID) (AccountData, error)
	ReadAccountByPublicKey(ctx context.Context, pubKey string) (AccountData, error)
	ListAccounts(ctx context.Context, operatorID OperatorID, filter paginate.PageFilter[AccountID]) ([]AccountData, error)
	DeleteAccount(ctx context.Context, id AccountID) error // must cascade delete users and nkeys
	CountOwnerAccounts(ctx context.Context, owner string) (int64, error)
}

type UserRepository interface {
	CreateUser(ctx context.Context, user UserData) error
	UpdateUser(ctx context.Context, user UserData) error
	ReadUser(ctx context.Context, id UserID) (UserData, error)
	ReadUserByName(ctx context.Context, operatorID OperatorID, accountID AccountID, name string) (UserData, error)
	ListUsers(ctx context.Context, accountID AccountID, filter paginate.PageFilter[UserID]) ([]UserData, error)
	DeleteUser(ctx context.Context, id UserID) error // must cascade delete nkeys
	CreateUserJWTIssuance(ctx context.Context, userID UserID, iss UserJWTIssuance) error
	ListUserJWTIssuances(ctx context.Context, userID UserID, filter paginate.PageFilter[int64]) ([]UserJWTIssuance, error)
	CountOwnerUsers(ctx context.Context, owner string) (int64, error)
}

type NkeyRepository interface {
	ReadNkey(ctx context.Context, id string, signingKey bool) (NkeyData, error)
	CreateNkey(ctx context.Context, nkey NkeyData) error
}
