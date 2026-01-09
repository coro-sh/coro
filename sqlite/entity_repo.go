package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/joshjon/kit/errtag"
	"github.com/joshjon/kit/paginate"
	"github.com/joshjon/kit/ref"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/sqlite/sqlc"
	"github.com/coro-sh/coro/tx"
)

var _ entity.Repository = (*EntityRepository)(nil)

type EntityRepository struct {
	db   *sqlc.Queries
	txer *tx.SQLiteRepositoryTxer[entity.Repository]
}

func NewEntityRepository(db DB) *EntityRepository {
	return &EntityRepository{
		db: sqlc.New(db),
		txer: tx.NewSQLiteRepositoryTxer(db, tx.SQLiteRepositoryTxerConfig[entity.Repository]{
			Timeout: tx.DefaultTimeout,
			WithTxFunc: func(repo entity.Repository, txer *tx.SQLiteRepositoryTxer[entity.Repository], tx *sql.Tx) entity.Repository {
				cpy := *repo.(*EntityRepository)
				cpy.db = cpy.db.WithTx(tx)
				cpy.txer = txer
				return entity.Repository(&cpy)
			},
		}),
	}
}

func (r *EntityRepository) CreateNamespace(ctx context.Context, namespace *entity.Namespace) error {
	err := r.db.CreateNamespace(ctx, sqlc.CreateNamespaceParams{
		ID:    namespace.ID.String(),
		Name:  namespace.Name,
		Owner: namespace.Owner,
	})
	return tagEntityErr[entity.Namespace](err)
}

func (r *EntityRepository) ReadNamespace(ctx context.Context, id entity.NamespaceID) (*entity.Namespace, error) {
	ns, err := r.db.ReadNamespace(ctx, id.String())
	if err != nil {
		return nil, tagEntityErr[entity.Namespace](err)
	}
	return unmarshalNamespace(ns), nil
}

func (r *EntityRepository) BatchReadNamespaces(ctx context.Context, ids []entity.NamespaceID) ([]*entity.Namespace, error) {
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = id.String()
	}
	namespaces, err := r.db.BatchReadNamespaces(ctx, idStrs)
	if err != nil {
		return nil, tagEntityErr[entity.Namespace](err)
	}
	return unmarshalList(namespaces, unmarshalNamespace), nil
}

func (r *EntityRepository) ReadNamespaceByName(ctx context.Context, name string, owner string) (*entity.Namespace, error) {
	ns, err := r.db.ReadNamespaceByName(ctx, sqlc.ReadNamespaceByNameParams{
		Name:  name,
		Owner: owner,
	})
	if err != nil {
		return nil, tagEntityErr[entity.Namespace](err)
	}
	return unmarshalNamespace(ns), nil
}

func (r *EntityRepository) ListNamespaces(ctx context.Context, owner string, filter paginate.PageFilter[entity.NamespaceID]) ([]*entity.Namespace, error) {
	params := sqlc.ListNamespacesParams{
		Owner: owner,
		Size:  int64(filter.Size),
	}
	if filter.Cursor != nil {
		params.Cursor = ref.Ptr(filter.Cursor.String())
	}
	namespaces, err := r.db.ListNamespaces(ctx, params)
	if err != nil {
		return nil, err
	}

	return unmarshalList(namespaces, unmarshalNamespace), nil
}

func (r *EntityRepository) DeleteNamespace(ctx context.Context, id entity.NamespaceID) error {
	err := r.db.DeleteNamespace(ctx, id.String())
	return tagEntityErr[entity.Namespace](err)
}

func (r *EntityRepository) CountOwnerNamespaces(ctx context.Context, owner string) (int64, error) {
	namespaces, err := r.db.CountOwnerNamespaces(ctx, owner)
	return namespaces, tagErr(err)
}

func (r *EntityRepository) CreateOperator(ctx context.Context, operator entity.OperatorData) error {
	err := r.db.CreateOperator(ctx, sqlc.CreateOperatorParams{
		ID:          operator.ID.String(),
		NamespaceID: operator.NamespaceID.String(),
		Name:        operator.Name,
		PublicKey:   operator.PublicKey,
		Jwt:         operator.JWT,
	})
	return tagEntityErr[entity.Operator](err)
}

func (r *EntityRepository) UpdateOperator(ctx context.Context, operator entity.OperatorData) error {
	err := r.db.UpdateOperator(ctx, sqlc.UpdateOperatorParams{
		ID:   operator.ID.String(),
		Name: operator.Name,
		Jwt:  operator.JWT,
	})
	return tagEntityErr[entity.Operator](err)
}

func (r *EntityRepository) ReadOperator(ctx context.Context, id entity.OperatorID) (entity.OperatorData, error) {
	op, err := r.db.ReadOperator(ctx, id.String())
	if err != nil {
		return entity.OperatorData{}, tagEntityErr[entity.Operator](err)
	}

	return unmarshalOperator(op), nil
}

func (r *EntityRepository) ReadOperatorByName(ctx context.Context, name string) (entity.OperatorData, error) {
	op, err := r.db.ReadOperatorByName(ctx, name)
	if err != nil {
		return entity.OperatorData{}, tagEntityErr[entity.Operator](err)
	}

	return unmarshalOperator(op), nil
}

func (r *EntityRepository) ReadOperatorByPublicKey(ctx context.Context, pubKey string) (entity.OperatorData, error) {
	op, err := r.db.ReadOperatorByPublicKey(ctx, pubKey)
	if err != nil {
		return entity.OperatorData{}, tagEntityErr[entity.Operator](err)
	}

	return unmarshalOperator(op), nil
}

func (r *EntityRepository) ListOperators(ctx context.Context, namespaceID entity.NamespaceID, filter paginate.PageFilter[entity.OperatorID]) ([]entity.OperatorData, error) {
	params := sqlc.ListOperatorsParams{
		NamespaceID: namespaceID.String(),
		Size:        int64(filter.Size),
	}
	if filter.Cursor != nil {
		params.Cursor = ref.Ptr(filter.Cursor.String())
	}

	ops, err := r.db.ListOperators(ctx, params)
	if err != nil {
		return nil, tagEntityErr[entity.Namespace](err)
	}

	return unmarshalList(ops, unmarshalOperator), nil
}

func (r *EntityRepository) DeleteOperator(ctx context.Context, id entity.OperatorID) error {
	err := r.db.DeleteOperator(ctx, id.String())
	return tagEntityErr[entity.Operator](err)
}

func (r *EntityRepository) CountOwnerOperators(ctx context.Context, owner string) (int64, error) {
	ops, err := r.db.CountOwnerOperators(ctx, owner)
	return ops, tagErr(err)
}

func (r *EntityRepository) CreateAccount(ctx context.Context, account entity.AccountData) error {
	params := sqlc.CreateAccountParams{
		ID:          account.ID.String(),
		NamespaceID: account.NamespaceID.String(),
		OperatorID:  account.OperatorID.String(),
		Name:        account.Name,
		PublicKey:   account.PublicKey,
		Jwt:         account.JWT,
	}
	if account.UserJWTDuration != nil {
		params.UserJwtDuration = ref.Ptr(int64(account.UserJWTDuration.Seconds()))
	}
	err := r.db.CreateAccount(ctx, params)
	return tagEntityErr[entity.Account](err)
}

func (r *EntityRepository) UpdateAccount(ctx context.Context, account entity.AccountData) error {
	params := sqlc.UpdateAccountParams{
		ID:   account.ID.String(),
		Name: account.Name,
		Jwt:  account.JWT,
	}
	if account.UserJWTDuration != nil {
		params.UserJwtDuration = ref.Ptr(int64(account.UserJWTDuration.Seconds()))
	}
	err := r.db.UpdateAccount(ctx, params)
	return tagEntityErr[entity.Account](err)
}

func (r *EntityRepository) ReadAccount(ctx context.Context, id entity.AccountID) (entity.AccountData, error) {
	op, err := r.db.ReadAccount(ctx, id.String())
	if err != nil {
		return entity.AccountData{}, tagEntityErr[entity.Account](err)
	}
	return unmarshalAccount(op), nil
}

func (r *EntityRepository) ReadAccountByPublicKey(ctx context.Context, pubKey string) (entity.AccountData, error) {
	acc, err := r.db.ReadAccountByPublicKey(ctx, pubKey)
	if err != nil {
		return entity.AccountData{}, tagEntityErr[entity.Account](err)
	}
	return unmarshalAccount(acc), nil
}

func (r *EntityRepository) ListAccounts(ctx context.Context, operatorID entity.OperatorID, filter paginate.PageFilter[entity.AccountID]) ([]entity.AccountData, error) {
	params := sqlc.ListAccountsParams{
		OperatorID: operatorID.String(),
		Size:       int64(filter.Size),
	}
	if filter.Cursor != nil {
		params.Cursor = ref.Ptr(filter.Cursor.String())
	}

	accounts, err := r.db.ListAccounts(ctx, params)
	if err != nil {
		return nil, tagEntityErr[entity.Operator](err)
	}

	return unmarshalList(accounts, unmarshalAccount), nil
}

func (r *EntityRepository) DeleteAccount(ctx context.Context, id entity.AccountID) error {
	err := r.db.DeleteAccount(ctx, id.String())
	return tagEntityErr[entity.Account](err)
}

func (r *EntityRepository) CountOwnerAccounts(ctx context.Context, owner string) (int64, error) {
	accs, err := r.db.CountOwnerAccounts(ctx, owner)
	return accs, tagErr(err)
}

func (r *EntityRepository) CreateUser(ctx context.Context, user entity.UserData) error {
	params := sqlc.CreateUserParams{
		ID:          user.ID.String(),
		NamespaceID: user.NamespaceID.String(),
		OperatorID:  user.OperatorID.String(),
		AccountID:   user.AccountID.String(),
		Name:        user.Name,
		Jwt:         user.JWT,
	}
	if user.JWTDuration != nil {
		params.JwtDuration = ref.Ptr(int64(user.JWTDuration.Seconds()))
	}
	err := r.db.CreateUser(ctx, params)
	return tagEntityErr[entity.User](err)
}

func (r *EntityRepository) UpdateUser(ctx context.Context, user entity.UserData) error {
	params := sqlc.UpdateUserParams{
		ID:   user.ID.String(),
		Name: user.Name,
		Jwt:  user.JWT,
	}
	if user.JWTDuration != nil {
		params.JwtDuration = ref.Ptr(int64(user.JWTDuration.Seconds()))
	}
	err := r.db.UpdateUser(ctx, params)
	return tagEntityErr[entity.User](err)
}

func (r *EntityRepository) ReadUser(ctx context.Context, id entity.UserID) (entity.UserData, error) {
	usr, err := r.db.ReadUser(ctx, id.String())
	if err != nil {
		return entity.UserData{}, tagEntityErr[entity.User](err)
	}
	return unmarshalUser(usr), nil
}

func (r *EntityRepository) ReadUserByName(ctx context.Context, operatorID entity.OperatorID, accountID entity.AccountID, name string) (entity.UserData, error) {
	usr, err := r.db.ReadUserByName(ctx, sqlc.ReadUserByNameParams{
		OperatorID: operatorID.String(),
		AccountID:  accountID.String(),
		Name:       name,
	})
	if err != nil {
		return entity.UserData{}, tagEntityErr[entity.User](err)
	}
	return unmarshalUser(usr), nil
}

func (r *EntityRepository) ListUsers(ctx context.Context, accountID entity.AccountID, filter paginate.PageFilter[entity.UserID]) ([]entity.UserData, error) {
	params := sqlc.ListUsersParams{
		AccountID: accountID.String(),
		Size:      int64(filter.Size),
	}
	if filter.Cursor != nil {
		params.Cursor = ref.Ptr(filter.Cursor.String())
	}

	users, err := r.db.ListUsers(ctx, params)
	if err != nil {
		return nil, tagEntityErr[entity.Account](err)
	}

	return unmarshalList(users, unmarshalUser), nil
}

func (r *EntityRepository) DeleteUser(ctx context.Context, id entity.UserID) error {
	err := r.db.DeleteUser(ctx, id.String())
	return tagEntityErr[entity.Account](err)
}

func (r *EntityRepository) CountOwnerUsers(ctx context.Context, owner string) (int64, error) {
	users, err := r.db.CountOwnerUsers(ctx, owner)
	return users, tagErr(err)
}

func (r *EntityRepository) CreateUserJWTIssuance(ctx context.Context, userID entity.UserID, iss entity.UserJWTIssuance) error {
	return r.db.CreateUserJWTIssuance(ctx, sqlc.CreateUserJWTIssuanceParams{
		UserID:     userID.String(),
		IssueTime:  iss.IssueTime,
		ExpireTime: iss.ExpireTime,
	})
}

func (r *EntityRepository) ListUserJWTIssuances(ctx context.Context, userID entity.UserID, filter paginate.PageFilter[int64]) ([]entity.UserJWTIssuance, error) {
	issuances, err := r.db.ListUserJWTIssuances(ctx, sqlc.ListUserJWTIssuancesParams{
		UserID: userID.String(),
		Cursor: filter.Cursor,
		Size:   int64(filter.Size),
	})
	if err != nil {
		return nil, tagEntityErr[entity.User](err)
	}
	return unmarshalList(issuances, unmarshalUserJWTIssuances), nil
}

func (r *EntityRepository) CreateNkey(ctx context.Context, nkey entity.NkeyData) error {
	if nkey.SigningKey {
		err := r.db.CreateSigningKey(ctx, sqlc.CreateSigningKeyParams{
			ID:   nkey.ID,
			Type: nkey.Type.String(),
			Seed: nkey.Seed,
		})
		return tagNkeyErr(err, nkey.SigningKey)
	}

	err := r.db.CreateNkey(ctx, sqlc.CreateNkeyParams{
		ID:   nkey.ID,
		Type: nkey.Type.String(),
		Seed: nkey.Seed,
	})
	return tagNkeyErr(err, nkey.SigningKey)
}

func (r *EntityRepository) ReadNkey(ctx context.Context, id string, signingKey bool) (_ entity.NkeyData, err error) {
	if signingKey {
		sk, err := r.db.ReadSigningKey(ctx, id)
		if err != nil {
			return entity.NkeyData{}, tagNkeyErr(err, signingKey)
		}

		return unmarshalSigningKey(sk), nil
	}

	nk, err := r.db.ReadNkey(ctx, id)
	if err != nil {
		return entity.NkeyData{}, tagNkeyErr(err, signingKey)
	}

	return unmarshalNkey(nk), nil
}

func (r *EntityRepository) WithTx(tx tx.Tx) entity.Repository {
	return r.txer.WithTx(r, tx)
}

func (r *EntityRepository) BeginTxFunc(ctx context.Context, fn func(ctx context.Context, tx tx.Tx, repo entity.Repository) error) error {
	return r.txer.BeginTxFunc(ctx, r, fn)
}

func tagNkeyErr(err error, signingKey bool) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		if signingKey {
			return errtag.Tag[entity.ErrTagSigningKeyNotFound](err)
		}
		return errtag.Tag[entity.ErrTagNkeyNotFound](err)
	}
	if isSQLiteErrCode(err, sqlite3.SQLITE_CONSTRAINT, sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY) {
		if signingKey {
			return errtag.Tag[entity.ErrTagSigningKeyConflict](err)
		}
		return errtag.Tag[entity.ErrTagNkeyConflict](err)
	}
	return err
}

func tagEntityErr[T entity.Entity](err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return errtag.Tag[entity.ErrTagNotFound[T]](err)
	}
	if isSQLiteErrCode(err, sqlite3.SQLITE_CONSTRAINT, sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY) {
		return errtag.Tag[entity.ErrTagConflict[T]](err)
	}
	return err
}

func tagErr(err error) error {
	return tx.TagSQLiteTimeoutErr(err)
}

func isSQLiteErrCode(err error, codes ...int) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		for _, code := range codes {
			if sqliteErr.Code() == code {
				return true
			}
		}
	}
	return false
}
