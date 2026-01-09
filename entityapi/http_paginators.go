package entityapi

import (
	"context"
	"strconv"

	"github.com/joshjon/kit/paginate"
	"github.com/labstack/echo/v4"

	"github.com/coro-sh/coro/entity"
)

type Lister interface {
	ListNamespaces(ctx context.Context, owner string, filter paginate.PageFilter[entity.NamespaceID]) ([]*entity.Namespace, error)
	ListOperators(ctx context.Context, namespaceID entity.NamespaceID, filter paginate.PageFilter[entity.OperatorID]) ([]*entity.Operator, error)
	ListAccounts(ctx context.Context, operatorID entity.OperatorID, filter paginate.PageFilter[entity.AccountID]) ([]*entity.Account, error)
	ListUsers(ctx context.Context, accountID entity.AccountID, filter paginate.PageFilter[entity.UserID]) ([]*entity.User, error)
	ListUserJWTIssuances(ctx context.Context, userID entity.UserID, filter paginate.PageFilter[int64]) ([]entity.UserJWTIssuance, error)
}

func PaginateNamespaces(ctx context.Context, c echo.Context, store Lister, owner string) ([]*entity.Namespace, string, error) {
	cursorGetter := func(item *entity.Namespace) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[entity.NamespaceID]) ([]*entity.Namespace, error) {
		return store.ListNamespaces(ctx, owner, filter)
	}
	return paginate.Paginate(c, paginate.Config[*entity.Namespace, entity.NamespaceID]{
		CursorParser: paginate.IDCursorParser[entity.NamespaceID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateOperators(ctx context.Context, c echo.Context, store Lister, nsID entity.NamespaceID) ([]*entity.Operator, string, error) {
	cursorGetter := func(item *entity.Operator) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[entity.OperatorID]) ([]*entity.Operator, error) {
		return store.ListOperators(ctx, nsID, filter)
	}
	return paginate.Paginate(c, paginate.Config[*entity.Operator, entity.OperatorID]{
		CursorParser: paginate.IDCursorParser[entity.OperatorID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateAccounts(ctx context.Context, c echo.Context, store Lister, opID entity.OperatorID) ([]*entity.Account, string, error) {
	cursorGetter := func(item *entity.Account) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[entity.AccountID]) ([]*entity.Account, error) {
		return store.ListAccounts(ctx, opID, filter)
	}
	return paginate.Paginate(c, paginate.Config[*entity.Account, entity.AccountID]{
		CursorParser: paginate.IDCursorParser[entity.AccountID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateUsers(ctx context.Context, c echo.Context, store Lister, accID entity.AccountID) ([]*entity.User, string, error) {
	cursorGetter := func(item *entity.User) string {
		return item.ID.String()
	}
	lister := func(filter paginate.PageFilter[entity.UserID]) ([]*entity.User, error) {
		return store.ListUsers(ctx, accID, filter)
	}
	return paginate.Paginate(c, paginate.Config[*entity.User, entity.UserID]{
		CursorParser: paginate.IDCursorParser[entity.UserID](),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}

func PaginateUserJWTIssuances(ctx context.Context, c echo.Context, store Lister, userID entity.UserID) ([]entity.UserJWTIssuance, string, error) {
	cursorGetter := func(item entity.UserJWTIssuance) string {
		return strconv.FormatInt(item.IssueTime, 10)
	}
	lister := func(filter paginate.PageFilter[int64]) ([]entity.UserJWTIssuance, error) {
		return store.ListUserJWTIssuances(ctx, userID, filter)
	}
	return paginate.Paginate(c, paginate.Config[entity.UserJWTIssuance, int64]{
		CursorParser: paginate.Int64CursorParser(),
		CursorGetter: cursorGetter,
		Lister:       lister,
	})
}
