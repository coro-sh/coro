package entity

import (
	"fmt"
	"strings"

	"github.com/cohesivestack/valgo"
	"github.com/labstack/echo/v4"

	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/log"
)

const namespaceContextKey = "req_namespace"

// NamespaceContextMiddleware extracts the NamespaceID from the request path and
// sets it in the handler context.
func NamespaceContextMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !isNamespacePath(c) {
				return next(c) // skip if path is not scoped to a namespace
			}
			nsIDStr := c.Param(pathParamNamespaceID)
			if err := valgo.In("params", valgo.Is(IDValidator[NamespaceID](nsIDStr, "namespace_id"))).Error(); err != nil {
				return err
			}
			nsID := MustParseID[NamespaceID](nsIDStr)
			c.Set(namespaceContextKey, nsID)
			c.Set(log.KeyNamespaceID, nsID)
			return next(c)
		}
	}
}

// InternalNamespaceMiddleware is a middleware to prevent access to the
// internal namespace.
func InternalNamespaceMiddleware(internalNamespaceID NamespaceID) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !isNamespacePath(c) {
				return next(c) // skip if path is not scoped to a namespace
			}
			nsID, err := NamespaceIDFromContext(c)
			if err != nil {
				return err
			}
			if nsID == internalNamespaceID {
				return errtag.NewTagged[errtag.Unauthorized]("namespace is reserved for internal use only")
			}

			return next(c)
		}
	}
}

func NamespaceIDFromContext(c echo.Context) (NamespaceID, error) {
	nsID, ok := c.Get(namespaceContextKey).(NamespaceID)
	if !ok {
		return NamespaceID{}, errtag.NewTagged[errtag.InvalidArgument]("namespace id not found in context", errtag.WithMsg("Namespace ID not found in request path"))
	}
	return nsID, nil
}

func isNamespacePath(c echo.Context) bool {
	return strings.HasPrefix(c.Path(), fmt.Sprintf("/api%s/namespaces/:%s", versionPath, pathParamNamespaceID))
}
