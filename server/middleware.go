package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/cohesivestack/valgo"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/internal/valgoutil"
	"github.com/coro-sh/coro/log"
)

// RequestLoggerConfigFunc configures request logging middleware on a Server.
type RequestLoggerConfigFunc func(logger log.Logger) middleware.RequestLoggerConfig

func newRequestLoggerConfig(logger log.Logger) middleware.RequestLoggerConfig {
	return middleware.RequestLoggerConfig{
		Skipper: func(c echo.Context) bool {
			return c.IsWebSocket()
		},
		LogValuesFunc:    logValuesFunc(logger),
		LogLatency:       true,
		LogRemoteIP:      true,
		LogMethod:        true,
		LogURI:           true,
		LogStatus:        true,
		LogError:         true,
		LogContentLength: true,
		LogResponseSize:  true,
	}
}

func logValuesFunc(logger log.Logger) func(c echo.Context, v middleware.RequestLoggerValues) error {
	return func(c echo.Context, v middleware.RequestLoggerValues) error {
		if v.Method == http.MethodOptions {
			return nil
		}

		meta := getDefaultMeta(c, v)

		level := slog.LevelInfo
		message := "request"
		if v.Error != nil {
			message = "request error"
			level = slog.LevelError
			var herr HTTPError
			if errors.As(v.Error, &herr) {
				meta["http_error"] = herr.Error()
				meta["error"] = herr.Internal
			} else {
				meta["error"] = v.Error.Error()
			}
		}

		l := logger
		keys := sortedMetaKeys(meta)
		for _, key := range keys {
			l = l.With(key, meta[key])
		}

		l.Log(c.Request().Context(), level, message)

		return v.Error
	}
}

func getDefaultMeta(c echo.Context, v middleware.RequestLoggerValues) map[any]any {
	defaultMeta := map[any]any{
		"time":          v.StartTime.UTC(),
		"method":        v.Method,
		"uri":           v.URI,
		"status":        v.Status,
		"latency_ms":    v.Latency.Milliseconds(),
		"latency_human": v.Latency.String(),
		"request_size":  v.ContentLength,
		"response_size": v.ResponseSize,
		"remote_ip":     v.RemoteIP,
	}

	for _, key := range log.HTTPKeys {
		if val := c.Get(key); val != nil {
			defaultMeta[key] = val
		}
	}

	return defaultMeta
}

func errorTransformMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err == nil {
			return nil
		}

		var echoErr *echo.HTTPError
		if errors.As(err, &echoErr) {
			msg := http.StatusText(echoErr.Code)
			if echoErr.Message != nil {
				msg = fmt.Sprintf("%v", echoErr.Message)
			}
			return HTTPError{
				Code:     echoErr.Code,
				Internal: echoErr.Error(),
				Message:  msg,
			}
		}

		var verr *valgo.Error
		var herr errtag.Tagger

		switch {
		case errors.As(err, &verr):
			// Bad request
			detailsStr := strings.Join(valgoutil.GetDetails(verr), "; ")
			formattedErr := fmt.Errorf("validate %s: %s", "request", detailsStr)
			herr = errtag.Tag[errtag.InvalidArgument](formattedErr, errtag.WithDetails(valgoutil.GetDetails(verr)...))
		case !errors.As(err, &herr):
			// Internal server error
			herr = errtag.Tag[errtag.Unknown](err)
		}

		return HTTPError{
			Code:     herr.Code(),
			Internal: herr.Error(),
			Message:  herr.Msg(),
			Details:  herr.Details(),
		}
	}
}

func httpErrorHandlerFunc(logger log.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		var herr HTTPError
		if !errors.As(err, &herr) {
			herr.Code = http.StatusInternalServerError
			herr.Message = http.StatusText(http.StatusInternalServerError)
			if err != nil { // safeguard
				herr.Internal = err.Error()
			}
		}
		if err = SetResponseError(c, herr.Code, herr); err != nil {
			logger.Error("failed to set response error", "error", err, "http_error", herr)
		}
	}
}

func sortedMetaKeys(meta map[any]any) []any {
	keys := make([]any, 0, len(meta))
	for key := range meta {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprintf("%v", keys[i]) < fmt.Sprintf("%v", keys[j])
	})

	return keys
}
