package entity

import (
	"context"
	"strconv"

	"github.com/joshjon/kit/paginate"
	"github.com/labstack/echo/v4"
)

type Lister interface {
	ListNamespaces(ctx context.Context, owner string, filter paginate.PageFilter[NamespaceID]) ([]*Namespace, error)
	ListOperators(ctx context.Context, namespaceID NamespaceID, filter paginate.PageFilter[OperatorID]) ([]*Operator, error)
	ListAccounts(ctx context.Context, operatorID OperatorID, filter paginate.PageFilter[AccountID]) ([]*Account, error)
	ListUsers(ctx context.Context, accountID AccountID, filter paginate.PageFilter[UserID]) ([]*User, error)
	ListUserJWTIssuances(ctx context.Context, userID UserID, filter paginate.PageFilter[int64]) ([]UserJWTIssuance, error)
}

func PaginateNamespaces(ctx context.Context, c echo.Context, store Lister, owner string) ([]*Namespace, string, error) {
	cursorGetter := func(item *Namespace) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[NamespaceID]) ([]*Namespace, error) {
		return store.ListNamespaces(ctx, owner, filter)
	}
	return paginate.Paginate(c, paginate.Config[*Namespace, NamespaceID]{
		CursorParser: paginate.IDCursorParser[NamespaceID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateOperators(ctx context.Context, c echo.Context, store Lister, nsID NamespaceID) ([]*Operator, string, error) {
	cursorGetter := func(item *Operator) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[OperatorID]) ([]*Operator, error) {
		return store.ListOperators(ctx, nsID, filter)
	}
	return paginate.Paginate(c, paginate.Config[*Operator, OperatorID]{
		CursorParser: paginate.IDCursorParser[OperatorID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateAccounts(ctx context.Context, c echo.Context, store Lister, opID OperatorID) ([]*Account, string, error) {
	cursorGetter := func(item *Account) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[AccountID]) ([]*Account, error) {
		return store.ListAccounts(ctx, opID, filter)
	}
	return paginate.Paginate(c, paginate.Config[*Account, AccountID]{
		CursorParser: paginate.IDCursorParser[AccountID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateUsers(ctx context.Context, c echo.Context, store Lister, accID AccountID) ([]*User, string, error) {
	cursorGetter := func(item *User) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[UserID]) ([]*User, error) {
		return store.ListUsers(ctx, accID, filter)
	}
	return paginate.Paginate(c, paginate.Config[*User, UserID]{
		CursorParser: paginate.IDCursorParser[UserID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateUserJWTIssuances(ctx context.Context, c echo.Context, store Lister, userID UserID) ([]UserJWTIssuance, string, error) {
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
