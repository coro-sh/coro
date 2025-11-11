package entity

import (
	"context"
	"time"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/encrypt"
	"github.com/coro-sh/coro/paginate"
	"github.com/coro-sh/coro/tx"
)

type TxStorer[S Storer] interface {
	Storer
	BeginTxFunc(ctx context.Context, fn func(ctx context.Context, store S) error) error
}

type Storer interface {
	NamespaceStorer
	OperatorStorer
	AccountStorer
	UserStorer
	NkeyStorer
	GetRepository() Repository
	SetRepository(repo Repository)
}

type NamespaceStorer interface {
	CreateNamespace(ctx context.Context, namespace *Namespace) error
	ReadNamespace(ctx context.Context, id NamespaceID) (*Namespace, error)
	ReadNamespaceByName(ctx context.Context, name, owner string) (*Namespace, error)
	BatchReadNamespaces(ctx context.Context, ids []NamespaceID) ([]*Namespace, error)
	ListNamespaces(ctx context.Context, owner string, filter paginate.PageFilter[NamespaceID]) ([]*Namespace, error)
	CountOwnerNamespaces(ctx context.Context, owner string) (int64, error)
	DeleteNamespace(ctx context.Context, id NamespaceID) error
}

type OperatorStorer interface {
	CreateOperator(ctx context.Context, operator *Operator) error
	UpdateOperator(ctx context.Context, operator *Operator) error
	ReadOperator(ctx context.Context, id OperatorID) (*Operator, error)
	ReadOperatorByName(ctx context.Context, name string) (*Operator, error)
	ReadOperatorByPublicKey(ctx context.Context, pubKey string) (*Operator, error)
	ListOperators(ctx context.Context, namespaceID NamespaceID, filter paginate.PageFilter[OperatorID]) ([]*Operator, error)
	DeleteOperator(ctx context.Context, id OperatorID) error
	CountOwnerOperators(ctx context.Context, owner string) (int64, error)
}

type AccountStorer interface {
	CreateAccount(ctx context.Context, account *Account) error
	UpdateAccount(ctx context.Context, account *Account) error
	ReadAccount(ctx context.Context, id AccountID) (*Account, error)
	ReadAccountByPublicKey(ctx context.Context, pubKey string) (*Account, error)
	ListAccounts(ctx context.Context, operatorID OperatorID, filter paginate.PageFilter[AccountID]) ([]*Account, error)
	DeleteAccount(ctx context.Context, id AccountID) error
	CountOwnerAccounts(ctx context.Context, owner string) (int64, error)
}

type UserStorer interface {
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	ReadUser(ctx context.Context, id UserID) (*User, error)
	ReadSystemUser(ctx context.Context, operatorID OperatorID, sysAccountID AccountID) (*User, error)
	ListUsers(ctx context.Context, accountID AccountID, filter paginate.PageFilter[UserID]) ([]*User, error)
	DeleteUser(ctx context.Context, id UserID) error
	CountOwnerUsers(ctx context.Context, owner string) (int64, error)
	RecordUserJWTIssuance(ctx context.Context, user *User) error
	ListUserJWTIssuances(ctx context.Context, userID UserID, filter paginate.PageFilter[int64]) ([]UserJWTIssuance, error)
}

type NkeyStorer interface {
	CreateNkey(ctx context.Context, nkey *Nkey) error
	ReadNkey(ctx context.Context, id string, signingKey bool) (*Nkey, error)
}

// StoreOption configures a Store during initialization.
type StoreOption func(store *Store)

// WithEncryption sets an Encrypter for the Store to encrypt sensitive data
// such as Nkey seeds.
func WithEncryption(encrypter encrypt.Encrypter) StoreOption {
	return func(store *Store) {
		store.encrypter = encrypter
	}
}

var _ TxStorer[*Store] = (*Store)(nil)

// Store is a convenience wrapper around a Repository to easily perform CRUD
// operations on entities and their associated nkeys.
type Store struct {
	repo      Repository
	encrypter encrypt.Encrypter
}

// NewStore creates a new Store instance with the provided Repository and
// options.
func NewStore(repo Repository, opts ...StoreOption) *Store {
	s := &Store{
		repo: repo,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CreateNamespace creates a Namespace.
func (s *Store) CreateNamespace(ctx context.Context, namespace *Namespace) error {
	return s.repo.CreateNamespace(ctx, namespace)
}

// ReadNamespace reads a Namespace by ID.
func (s *Store) ReadNamespace(ctx context.Context, id NamespaceID) (*Namespace, error) {
	return s.repo.ReadNamespace(ctx, id)
}

// ReadNamespaceByName reads a Namespace by name.
func (s *Store) ReadNamespaceByName(ctx context.Context, name string, owner string) (*Namespace, error) {
	return s.repo.ReadNamespaceByName(ctx, name, owner)
}

// BatchReadNamespaces reads a batch of Namespaces by IDs.
func (s *Store) BatchReadNamespaces(ctx context.Context, ids []NamespaceID) ([]*Namespace, error) {
	return s.repo.BatchReadNamespaces(ctx, ids)
}

// ListNamespaces reads a list of Namespaces matching the provided PageFilter.
func (s *Store) ListNamespaces(ctx context.Context, owner string, filter paginate.PageFilter[NamespaceID]) ([]*Namespace, error) {
	return s.repo.ListNamespaces(ctx, owner, filter)
}

// CountOwnerNamespaces returns the number of namespaces belonging to an owner.
func (s *Store) CountOwnerNamespaces(ctx context.Context, owner string) (int64, error) {
	return s.repo.CountOwnerNamespaces(ctx, owner)
}

// DeleteNamespace deletes a of Namespace.
func (s *Store) DeleteNamespace(ctx context.Context, id NamespaceID) error {
	return s.repo.DeleteNamespace(ctx, id)
}

// CreateOperator creates an Operator and its associated nkeys in the store.
func (s *Store) CreateOperator(ctx context.Context, operator *Operator) (err error) {
	data, err := operator.Data()
	if err != nil {
		return err
	}

	return s.BeginTxFunc(ctx, func(ctx context.Context, store *Store) error {
		if err = store.CreateNkey(ctx, operator.NKey()); err != nil {
			return err
		}

		if err = store.CreateNkey(ctx, operator.SigningKey()); err != nil {
			return err
		}

		return store.repo.CreateOperator(ctx, data)
	})
}

// UpdateOperator updates an existing Operator in the store.
func (s *Store) UpdateOperator(ctx context.Context, operator *Operator) error {
	data, err := operator.Data()
	if err != nil {
		return err
	}

	return s.repo.UpdateOperator(ctx, data)
}

// ReadOperator reads an Operator from the store by its ID.
func (s *Store) ReadOperator(ctx context.Context, id OperatorID) (*Operator, error) {
	data, err := s.repo.ReadOperator(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.initOperator(ctx, data)
}

// ReadOperatorByName reads an Operator from the store by its name.
func (s *Store) ReadOperatorByName(ctx context.Context, name string) (*Operator, error) {
	data, err := s.repo.ReadOperatorByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return s.initOperator(ctx, data)
}

// ReadOperatorByPublicKey reads an Operator from the store using its public key.
func (s *Store) ReadOperatorByPublicKey(ctx context.Context, pubKey string) (*Operator, error) {
	data, err := s.repo.ReadOperatorByPublicKey(ctx, pubKey)
	if err != nil {
		return nil, err
	}

	return s.initOperator(ctx, data)
}

// ListOperators reads a list of Operators for a specific Namespace, matching the
// provided PageFilter.
func (s *Store) ListOperators(ctx context.Context, namespaceID NamespaceID, filter paginate.PageFilter[OperatorID]) ([]*Operator, error) {
	data, err := s.repo.ListOperators(ctx, namespaceID, filter)
	if err != nil {
		return nil, err
	}

	operators := make([]*Operator, len(data))
	for i, od := range data {
		op, err := s.initOperator(ctx, od)
		if err != nil {
			return nil, err
		}
		operators[i] = op
	}

	return operators, nil
}

// DeleteOperator deletes an Operator from the store by its ID.
func (s *Store) DeleteOperator(ctx context.Context, id OperatorID) (err error) {
	return s.repo.DeleteOperator(ctx, id)
}

// CountOwnerOperators returns the number of operators belonging to an owner.
func (s *Store) CountOwnerOperators(ctx context.Context, owner string) (int64, error) {
	return s.repo.CountOwnerOperators(ctx, owner)
}

// CreateAccount creates an Account and its associated Nkeys to the store.
func (s *Store) CreateAccount(ctx context.Context, account *Account) (err error) {
	data, err := account.Data()
	if err != nil {
		return err
	}

	return s.BeginTxFunc(ctx, func(ctx context.Context, store *Store) error {
		if err = store.CreateNkey(ctx, account.NKey()); err != nil {
			return err
		}

		if err = store.CreateNkey(ctx, account.SigningKey()); err != nil {
			return err
		}

		return store.repo.CreateAccount(ctx, data)
	})
}

// UpdateAccount updates an existing Account in the store.
func (s *Store) UpdateAccount(ctx context.Context, account *Account) (err error) {
	data, err := account.Data()
	if err != nil {
		return err
	}

	return s.repo.UpdateAccount(ctx, data)
}

// ReadAccount reads an Account from the store by its ID.
func (s *Store) ReadAccount(ctx context.Context, id AccountID) (*Account, error) {
	data, err := s.repo.ReadAccount(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.initAccount(ctx, data)
}

// ReadAccountByPublicKey reads an Account from the store using its public key.
func (s *Store) ReadAccountByPublicKey(ctx context.Context, pubKey string) (*Account, error) {
	data, err := s.repo.ReadAccountByPublicKey(ctx, pubKey)
	if err != nil {
		return nil, err
	}
	return s.initAccount(ctx, data)
}

// ListAccounts reads a list of Accounts for a specific Operator, matching
// the provided PageFilter.
func (s *Store) ListAccounts(ctx context.Context, operatorID OperatorID, filter paginate.PageFilter[AccountID]) ([]*Account, error) {
	data, err := s.repo.ListAccounts(ctx, operatorID, filter)
	if err != nil {
		return nil, err
	}

	accounts := make([]*Account, len(data))
	for i, d := range data {
		cc, err := s.initAccount(ctx, d)
		if err != nil {
			return nil, err
		}
		accounts[i] = cc
	}

	return accounts, nil
}

// DeleteAccount deletes an Account from the store by its ID.
func (s *Store) DeleteAccount(ctx context.Context, id AccountID) (err error) {
	return s.repo.DeleteAccount(ctx, id)
}

// CountOwnerAccounts returns the number of accounts belonging to an owner.
func (s *Store) CountOwnerAccounts(ctx context.Context, owner string) (int64, error) {
	return s.repo.CountOwnerAccounts(ctx, owner)
}

// CreateUser creates a User and its associated Nkey in the store.
func (s *Store) CreateUser(ctx context.Context, user *User) (err error) {
	return s.BeginTxFunc(ctx, func(ctx context.Context, store *Store) error {
		if err = store.CreateNkey(ctx, user.NKey()); err != nil {
			return err
		}

		data, err := user.Data()
		if err != nil {
			return err
		}

		return store.repo.CreateUser(ctx, data)
	})
}

// UpdateUser updates an existing User in the store.
func (s *Store) UpdateUser(ctx context.Context, user *User) (err error) {
	data, err := user.Data()
	if err != nil {
		return err
	}

	return s.repo.UpdateUser(ctx, data)
}

// ReadUser reads a User from the store by its ID.
func (s *Store) ReadUser(ctx context.Context, id UserID) (*User, error) {
	data, err := s.repo.ReadUser(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.initUser(ctx, data)
}

// ReadSystemUser reads the system User for a given Operator and System Account.
func (s *Store) ReadSystemUser(ctx context.Context, operatorID OperatorID, sysAccountID AccountID) (*User, error) {
	data, err := s.repo.ReadUserByName(ctx, operatorID, sysAccountID, constants.SysUserName)
	if err != nil {
		return nil, err
	}

	return s.initUser(ctx, data)
}

// ListUsers reads a list of Users for a specific Account ID, matching the
// provided PageFilter.
func (s *Store) ListUsers(ctx context.Context, accountID AccountID, filter paginate.PageFilter[UserID]) ([]*User, error) {
	data, err := s.repo.ListUsers(ctx, accountID, filter)
	if err != nil {
		return nil, err
	}

	users := make([]*User, len(data))
	for i, d := range data {
		usr, err := s.initUser(ctx, d)
		if err != nil {
			return nil, err
		}
		users[i] = usr
	}

	return users, nil
}

// DeleteUser deletes a User from the store by its ID.
func (s *Store) DeleteUser(ctx context.Context, id UserID) (err error) {
	return s.repo.DeleteUser(ctx, id)
}

// CountOwnerUsers returns the number of accounts belonging to an owner.
func (s *Store) CountOwnerUsers(ctx context.Context, owner string) (int64, error) {
	return s.repo.CountOwnerUsers(ctx, owner)
}

func (s *Store) RecordUserJWTIssuance(ctx context.Context, user *User) error {
	claims, err := user.Claims()
	if err != nil {
		return err
	}
	return s.repo.CreateUserJWTIssuance(ctx, user.ID, UserJWTIssuance{
		IssueTime:  claims.IssuedAt,
		ExpireTime: claims.Expires,
	})
}

type UserJWTIssuance struct {
	IssueTime  int64 `json:"issue_time"`            // unix
	ExpireTime int64 `json:"expire_time,omitempty"` // unix
}

func (u UserJWTIssuance) IsActive() bool {
	if u.ExpireTime == 0 {
		return true
	}
	return time.Now().Unix() < u.ExpireTime
}

func (s *Store) ListUserJWTIssuances(ctx context.Context, userID UserID, filter paginate.PageFilter[int64]) ([]UserJWTIssuance, error) {
	return s.repo.ListUserJWTIssuances(ctx, userID, filter)
}

// CreateNkey creates an Nkey in the store. If encryption is enabled, the Nkey
// seed is encrypted before being saved.
func (s *Store) CreateNkey(ctx context.Context, nkey *Nkey) error {
	if s.encrypter == nil {
		return s.repo.CreateNkey(ctx, nkey.NkeyData)
	}

	encSeed, err := s.encrypter.Encrypt(ctx, nkey.Seed)
	if err != nil {
		return err
	}

	dataCpy := nkey.NkeyData
	dataCpy.Seed = encSeed
	return s.repo.CreateNkey(ctx, dataCpy)
}

// ReadNkey reads an Nkey by its ID from the store. If encryption is enabled,
// the Nkey seed is decrypted after being read.
func (s *Store) ReadNkey(ctx context.Context, id string, signingKey bool) (*Nkey, error) {
	data, err := s.repo.ReadNkey(ctx, id, signingKey)
	if err != nil {
		return nil, err
	}

	if s.encrypter != nil {
		data.Seed, err = s.encrypter.Decrypt(ctx, data.Seed)
		if err != nil {
			return nil, err
		}
	}

	return NewNkeyFromData(data)
}

// BeginTxFunc begins a transactional operation on the store.
func (s *Store) BeginTxFunc(ctx context.Context, fn func(ctx context.Context, store *Store) error) error {
	return s.repo.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo Repository) error {
		cpy := *s
		cpy.repo = repo
		return fn(ctx, &cpy)
	})
}

// SetRepository replaces the repository instance for the Store.
func (s *Store) SetRepository(repo Repository) {
	s.repo = repo
}

// GetRepository returns the repository instance for the Store.
func (s *Store) GetRepository() Repository {
	return s.repo
}

func (s *Store) initOperator(ctx context.Context, data OperatorData) (*Operator, error) {
	nk, err := s.ReadNkey(ctx, data.ID.String(), false)
	if err != nil {
		return nil, err
	}

	sk, err := s.ReadNkey(ctx, data.ID.String(), true)
	if err != nil {
		return nil, err
	}

	return NewOperatorFromData(nk, sk, data)
}

func (s *Store) initAccount(ctx context.Context, data AccountData) (*Account, error) {
	nk, err := s.ReadNkey(ctx, data.ID.String(), false)
	if err != nil {
		return nil, err
	}

	sk, err := s.ReadNkey(ctx, data.ID.String(), true)
	if err != nil {
		return nil, err
	}

	return NewAccountFromData(nk, sk, data)
}

func (s *Store) initUser(ctx context.Context, data UserData) (*User, error) {
	nk, err := s.ReadNkey(ctx, data.ID.String(), false)
	if err != nil {
		return nil, err
	}
	return NewUserFromData(nk, data)
}
