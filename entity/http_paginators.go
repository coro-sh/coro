package entity

import (
	"context"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.jetify.com/typeid"

	"github.com/coro-sh/coro/paginate"
)

func PaginateNamespaces(ctx context.Context, c echo.Context, store *Store) ([]*Namespace, string, error) {
	cursorGetter := func(item *Namespace) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[NamespaceID]) ([]*Namespace, error) {
		return store.ListNamespaces(ctx, filter)
	}
	return paginate.Paginate(c, paginate.Config[*Namespace, NamespaceID]{
		CursorParser: IDCursorParser[NamespaceID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateOperators(ctx context.Context, c echo.Context, store *Store, nsID NamespaceID) ([]*Operator, string, error) {
	cursorGetter := func(item *Operator) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[OperatorID]) ([]*Operator, error) {
		return store.ListOperators(ctx, nsID, filter)
	}
	return paginate.Paginate(c, paginate.Config[*Operator, OperatorID]{
		CursorParser: IDCursorParser[OperatorID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateAccounts(ctx context.Context, c echo.Context, store *Store, opID OperatorID) ([]*Account, string, error) {
	cursorGetter := func(item *Account) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[AccountID]) ([]*Account, error) {
		return store.ListAccounts(ctx, opID, filter)
	}
	return paginate.Paginate(c, paginate.Config[*Account, AccountID]{
		CursorParser: IDCursorParser[AccountID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateUsers(ctx context.Context, c echo.Context, store *Store, accID AccountID) ([]*User, string, error) {
	cursorGetter := func(item *User) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[UserID]) ([]*User, error) {
		return store.ListUsers(ctx, accID, filter)
	}
	return paginate.Paginate(c, paginate.Config[*User, UserID]{
		CursorParser: IDCursorParser[UserID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateUserJWTIssuances(ctx context.Context, c echo.Context, store *Store, userID UserID) ([]UserJWTIssuance, string, error) {
	cursorGetter := func(item UserJWTIssuance) string {
		return strconv.FormatInt(item.IssueTime, 10)
	}
	lister := func(filter paginate.PageFilter[int64]) ([]UserJWTIssuance, error) {
		return store.ListUserJWTIssuances(ctx, userID, filter)
	}
	return paginate.Paginate(c, paginate.Config[UserJWTIssuance, int64]{
		CursorParser: paginate.Int64CursorParser(),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func IDCursorParser[T ID, PT typeid.SubtypePtr[T]]() paginate.CursorParserFunc[T] {
	return func(rawCursor string) (*T, error) {
		entityID, err := ParseID[T, PT](rawCursor)
		if err != nil {
			return nil, err
		}
		return &entityID, nil
	}
}
