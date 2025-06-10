package paginate

import (
	"encoding/base64"
	"strconv"

	"github.com/cohesivestack/valgo"
	"github.com/labstack/echo/v4"
)

const (
	MaxPageSize          = int32(500)
	DefaultPageSize      = MaxPageSize
	PageSizeQueryParam   = "page_size"
	PageCursorQueryParam = "page_cursor"
)

type PageFilter[C comparable] struct {
	Size   int32
	Cursor *C
}

type CursorParserFunc[C comparable] func(rawCursor string) (*C, error)

type CursorGetterFunc[T any] func(item T) string

type ListFunc[T any, C comparable] func(filter PageFilter[C]) ([]T, error)

type Config[T any, C comparable] struct {
	CursorParser CursorParserFunc[C]
	CursorGetter CursorGetterFunc[T]
	Lister       ListFunc[T, C]
}

func Paginate[T any, C comparable](c echo.Context, config Config[T, C]) ([]T, string, error) {
	filter, err := pageFilterFromQueryParams[C](c, config.CursorParser)
	if err != nil {
		return nil, "", err
	}

	items, err := config.Lister(filter)
	if err != nil {
		return nil, "", err
	}

	cursor := ""
	if len(items) == int(filter.Size) {
		lastItem := items[len(items)-1]
		items = items[:len(items)-1]
		cursor = config.CursorGetter(lastItem)
	}

	return items, cursor, nil
}

func pageFilterFromQueryParams[C comparable](c echo.Context, cursorGetter CursorParserFunc[C]) (PageFilter[C], error) {
	const queryParamsTitle = "query_params"

	filter := PageFilter[C]{
		Size: DefaultPageSize,
	}

	sizeStr := c.QueryParam(PageSizeQueryParam)
	if sizeStr != "" {
		size64, err := strconv.ParseInt(sizeStr, 10, 32)
		if err != nil {
			return filter, err
		}

		filter.Size = int32(size64)
		verr := valgo.In(queryParamsTitle, valgo.Is(valgo.Int32(filter.Size, PageSizeQueryParam).Between(int32(1), MaxPageSize))).Error()
		if verr != nil {
			return filter, verr
		}
	}

	filter.Size++ // add one more so we can check if there is another page to return

	b64Cursor := c.QueryParam(PageCursorQueryParam)
	if b64Cursor != "" {
		verr := valgo.In(queryParamsTitle, valgo.AddErrorMessage(PageCursorQueryParam, "Must be a valid cursor")).Error()

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
