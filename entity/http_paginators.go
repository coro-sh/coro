package entity

import (
	"context"
	"encoding/base64"
	"strconv"

	"github.com/cohesivestack/valgo"
	"github.com/labstack/echo/v4"
	"go.jetify.com/typeid"
)

const (
	pageSizeQueryParam   = "page_size"
	pageCursorQueryParam = "page_cursor"
)

func PaginateNamespaces(ctx context.Context, c echo.Context, store *Store) ([]*Namespace, string, error) {
	cursorGetter := func(item *Namespace) string {
		return item.ID.String()
	}
	lister := func(filter PageFilter[NamespaceID]) ([]*Namespace, error) {
		return store.ListNamespaces(ctx, filter)
	}
	return paginate(c, entityIDCursorGetter[NamespaceID](), cursorGetter, lister)
}

func PaginateOperators(ctx context.Context, c echo.Context, store *Store, nsID NamespaceID) ([]*Operator, string, error) {
	cursorGetter := func(item *Operator) string {
		return item.ID.String()
	}
	lister := func(filter PageFilter[OperatorID]) ([]*Operator, error) {
		return store.ListOperators(ctx, nsID, filter)
	}
	return paginate(c, entityIDCursorGetter[OperatorID](), cursorGetter, lister)
}

func PaginateAccounts(ctx context.Context, c echo.Context, store *Store, opID OperatorID) ([]*Account, string, error) {
	cursorGetter := func(item *Account) string {
		return item.ID.String()
	}
	lister := func(filter PageFilter[AccountID]) ([]*Account, error) {
		return store.ListAccounts(ctx, opID, filter)
	}
	return paginate(c, entityIDCursorGetter[AccountID](), cursorGetter, lister)
}

func PaginateUsers(ctx context.Context, c echo.Context, store *Store, accID AccountID) ([]*User, string, error) {
	cursorGetter := func(item *User) string {
		return item.ID.String()
	}
	lister := func(filter PageFilter[UserID]) ([]*User, error) {
		return store.ListUsers(ctx, accID, filter)
	}
	return paginate(c, entityIDCursorGetter[UserID](), cursorGetter, lister)
}

func PaginateUserJWTIssuances(ctx context.Context, c echo.Context, store *Store, userID UserID) ([]UserJWTIssuance, string, error) {
	cursorGetter := func(item UserJWTIssuance) string {
		return strconv.FormatInt(item.IssueTime, 10)
	}
	lister := func(filter PageFilter[int64]) ([]UserJWTIssuance, error) {
		return store.ListUserJWTIssuances(ctx, userID, filter)
	}
	return paginate[UserJWTIssuance, int64](c, int64CursorGetter(), cursorGetter, lister)
}

type paginateCursorParserFunc[C comparable] func(rawCursor string) (*C, error)

type paginateCursorGetterFunc[T any] func(item T) string

type paginateListFunc[T any, C comparable] func(filter PageFilter[C]) ([]T, error)

func paginate[T any, C comparable](
	c echo.Context,
	cursorParserFn paginateCursorParserFunc[C],
	cursorGetterFn paginateCursorGetterFunc[T],
	listFn paginateListFunc[T, C],
) ([]T, string, error) {
	filter, err := pageFilterFromQueryParams[C](c, cursorParserFn)
	if err != nil {
		return nil, "", err
	}

	items, err := listFn(filter)
	if err != nil {
		return nil, "", err
	}

	cursor := ""
	if len(items) == int(filter.Size) {
		lastItem := items[len(items)-1]
		items = items[:len(items)-1]
		cursor = cursorGetterFn(lastItem)
	}

	return items, cursor, nil
}

func entityIDCursorGetter[T ID, PT typeid.SubtypePtr[T]]() paginateCursorParserFunc[T] {
	return func(rawCursor string) (*T, error) {
		entityID, err := ParseID[T, PT](rawCursor)
		if err != nil {
			return nil, err
		}
		return &entityID, nil
	}
}

func int64CursorGetter() paginateCursorParserFunc[int64] {
	return func(rawCursor string) (*int64, error) {
		num, err := strconv.ParseInt(rawCursor, 10, 64)
		if err != nil {
			return nil, err
		}
		return &num, nil
	}
}

func pageFilterFromQueryParams[C comparable](c echo.Context, cursorGetter paginateCursorParserFunc[C]) (PageFilter[C], error) {
	const queryParamsTitle = "query_params"

	filter := PageFilter[C]{
		Size: defaultPageSize,
	}

	sizeStr := c.QueryParam(pageSizeQueryParam)
	if sizeStr != "" {
		size64, err := strconv.ParseInt(sizeStr, 10, 32)
		if err != nil {
			return filter, err
		}

		filter.Size = int32(size64)
		verr := valgo.In(queryParamsTitle, valgo.Is(valgo.Int32(filter.Size, pageSizeQueryParam).Between(int32(1), maxPageSize))).Error()
		if verr != nil {
			return filter, verr
		}
	}

	filter.Size++ // add one more so we can check if there is another page to return

	b64Cursor := c.QueryParam(pageCursorQueryParam)
	if b64Cursor != "" {
		verr := valgo.In(queryParamsTitle, valgo.AddErrorMessage(pageCursorQueryParam, "Must be a valid cursor")).Error()

		cursor, err := base64.StdEncoding.DecodeString(b64Cursor)
		if err != nil {
			return filter, verr
		}

		filter.Cursor, err = cursorGetter(string(cursor))
		if err != nil {
			return filter, err
		}
	}

	return filter, nil
}
