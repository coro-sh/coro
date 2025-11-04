package entity

import (
	"context"
	"time"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/encrypt"
	"github.com/coro-sh/coro/paginate"
	"github.com/coro-sh/coro/tx"
)

// Repository is the interface for performing CRUD operations on Nkeys,
// Operators, Accounts, and Users.
type Repository interface {
	CreateNamespace(ctx context.Context, namespace *Namespace) error
	ReadNamespace(ctx context.Context, id NamespaceID) (*Namespace, error)
	ReadNamespaceByName(ctx context.Context, name string, owner string) (*Namespace, error)
	BatchReadNamespaces(ctx context.Context, ids []NamespaceID) ([]*Namespace, error)
	ListNamespaces(ctx context.Context, owner string, filter paginate.PageFilter[NamespaceID]) ([]*Namespace, error)
	DeleteNamespace(ctx context.Context, id NamespaceID) error // must cascade delete

	CreateOperator(ctx context.Context, operator OperatorData) error
	UpdateOperator(ctx context.Context, operator OperatorData) error
	ReadOperator(ctx context.Context, id OperatorID) (OperatorData, error)
	ReadOperatorByName(ctx context.Context, name string) (OperatorData, error)
	ReadOperatorByPublicKey(ctx context.Context, pubKey string) (OperatorData, error)
	ListOperators(ctx context.Context, namespaceID NamespaceID, filter paginate.PageFilter[OperatorID]) ([]OperatorData, error)
	DeleteOperator(ctx context.Context, id OperatorID) error // must cascade delete

	CreateAccount(ctx context.Context, account AccountData) error
	UpdateAccount(ctx context.Context, account AccountData) error
	ReadAccount(ctx context.Context, id AccountID) (AccountData, error)
	ReadAccountByPublicKey(ctx context.Context, pubKey string) (AccountData, error)
	ListAccounts(ctx context.Context, operatorID OperatorID, filter paginate.PageFilter[AccountID]) ([]AccountData, error)
	DeleteAccount(ctx context.Context, id AccountID) error // must cascade delete

	CreateUser(ctx context.Context, user UserData) error
	UpdateUser(ctx context.Context, user UserData) error
	ReadUser(ctx context.Context, id UserID) (UserData, error)
	ReadUserByName(ctx context.Context, operatorID OperatorID, accountID AccountID, name string) (UserData, error)
	ListUsers(ctx context.Context, accountID AccountID, filter paginate.PageFilter[UserID]) ([]UserData, error)
	DeleteUser(ctx context.Context, id UserID) error // must cascade delete

	CreateUserJWTIssuance(ctx context.Context, userID UserID, iss UserJWTIssuance) error
	ListUserJWTIssuances(ctx context.Context, userID UserID, filter paginate.PageFilter[int64]) ([]UserJWTIssuance, error)

	ReadNkey(ctx context.Context, id string, signingKey bool) (NkeyData, error)
	CreateNkey(ctx context.Context, nkey NkeyData) error

	WithTx(tx tx.Tx) (Repository, error)
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

// Store create, reads, and updates entities and their associated nkeys.
type Store struct {
	repo      Repository
	txer      tx.Txer
	encrypter encrypt.Encrypter
	isTx      bool
}

// NewStore creates a new Store instance with the provided Repository and
// optional configuration options.
func NewStore(txer tx.Txer, repo Repository, opts ...StoreOption) *Store {
	s := &Store{
		repo: repo,
		txer: txer,
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

	store := s
	if !s.isTx {
		// begin transaction if not already started
		txn, err := s.txer.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Handle(ctx, txn, &err)
		store, err = store.WithTx(txn)
		if err != nil {
			return err
		}
	}

	if err = store.CreateNkey(ctx, operator.NKey()); err != nil {
		return err
	}

	if err = store.CreateNkey(ctx, operator.SigningKey()); err != nil {
		return err
	}

	return store.repo.CreateOperator(ctx, data)
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

// CreateAccount creates an Account and its associated Nkeys to the store.
func (s *Store) CreateAccount(ctx context.Context, account *Account) (err error) {
	data, err := account.Data()
	if err != nil {
		return err
	}

	store := s
	if !s.isTx {
		// begin transaction if not already started
		txn, err := s.txer.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Handle(ctx, txn, &err)
		store, err = store.WithTx(txn)
		if err != nil {
			return err
		}
	}

	if err = store.CreateNkey(ctx, account.NKey()); err != nil {
		return err
	}

	if err = store.CreateNkey(ctx, account.SigningKey()); err != nil {
		return err
	}

	return store.repo.CreateAccount(ctx, data)
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

// CreateUser creates a User and its associated Nkey in the store.
func (s *Store) CreateUser(ctx context.Context, user *User) (err error) {
	store := s
	if !s.isTx {
		// begin transaction if not already started
		txn, err := s.txer.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Handle(ctx, txn, &err)
		store, err = store.WithTx(txn)
		if err != nil {
			return err
		}
	}

	if err = store.CreateNkey(ctx, user.NKey()); err != nil {
		return err
	}

	data, err := user.Data()
	if err != nil {
		return err
	}

	return store.repo.CreateUser(ctx, data)
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

// WithTx creates a new Store instance that uses the provided transaction.
func (s *Store) WithTx(txn tx.Tx) (*Store, error) {
	cpy := *s
	cpy.isTx = true
	repo, err := cpy.repo.WithTx(txn)
	cpy.repo = repo
	if err != nil {
		return nil, err
	}
	return &cpy, nil
}

// DoTx executes a transactional operation on the store.
func (s *Store) DoTx(ctx context.Context, txn tx.Tx, fn func(ctx context.Context, store *Store) error) error {
	cpy := *s
	cpy.isTx = true
	repo, err := cpy.repo.WithTx(txn)
	if err != nil {
		return err
	}
	cpy.repo = repo

	err = fn(ctx, &cpy)
	tx.Handle(context.Background(), txn, &err)
	return err
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
