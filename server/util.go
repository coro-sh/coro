package server

import (
	"encoding/base64"

	"github.com/labstack/echo/v4"
)

type validatable interface {
	Validate() error
}

func BindRequest[T validatable](c echo.Context) (T, error) {
	var req T
	if err := c.Bind(&req); err != nil {
		return req, err
	}
	if err := req.Validate(); err != nil {
		return req, err
	}
	return req, nil
}

func SetResponse[T any](c echo.Context, code int, data T) error {
	return c.JSON(code, &Response[T]{
		Data: data,
	})
}

func SetResponseList[T any](c echo.Context, code int, data []T, nextCursor string) error {
	res := &ResponseList[T]{
		Data: data,
	}
	if nextCursor != "" {
		b64Cursor := base64.URLEncoding.EncodeToString([]byte(nextCursor))
		res.NextPageCursor = &b64Cursor
	}
	return c.JSON(code, res)
}

func SetResponseError(c echo.Context, code int, err HTTPError) error {
	return c.JSON(code, &ResponseError{
		Error: err,
	})
}
