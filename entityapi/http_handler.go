package entityapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/joshjon/kit/errtag"
	"github.com/joshjon/kit/id"
	"github.com/joshjon/kit/ref"
	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"
	"github.com/nats-io/jwt/v2"
	"golang.org/x/sync/errgroup"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/logkey"
)

const (
	PathParamNamespaceID = "namespace_id"
	PathParamOperatorID  = "operator_id"
	PathParamAccountID   = "account_id"
	PathParamUserID      = "user_id"
)

// Commander sends commands to a NATS server.
type Commander interface {
	NotifyAccountClaimsUpdate(ctx context.Context, account *entity.Account) error
	NotifyAccountClaimsDelete(ctx context.Context, operator *entity.Operator, account *entity.Account) error
	// Ping checks if an Operator is connected and ready to receive commands.
	Ping(ctx context.Context, operatorID entity.OperatorID) (entity.OperatorNATSStatus, error)
}

// HTTPHandlerOption configures a HTTPHandler.
type HTTPHandlerOption[S entity.Storer] func(handler *HTTPHandler[S])

// WithCommander sets a Commander on the handler to enable commands.
func WithCommander[S entity.Storer](commander Commander) HTTPHandlerOption[S] {
	return func(handler *HTTPHandler[S]) {
		handler.commander = commander
	}
}

// WithNamespaceOwnerGetter sets a function to get the namespace owner for a
// request. If not set, the default namespace owner will be used.
func WithNamespaceOwnerGetter[S entity.Storer](fn func(c echo.Context) (string, error)) HTTPHandlerOption[S] {
	return func(handler *HTTPHandler[S]) {
		handler.ownerGetter = fn
	}
}

// HTTPHandler handles entity HTTP requests.
type HTTPHandler[S entity.Storer] struct {
	store       entity.TxStorer[S]
	commander   Commander
	ownerGetter func(c echo.Context) (string, error)
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler[S entity.Storer](store entity.TxStorer[S], opts ...HTTPHandlerOption[S]) *HTTPHandler[S] {
	h := &HTTPHandler[S]{
		store: store,
		ownerGetter: func(c echo.Context) (string, error) {
			return constants.DefaultNamespaceOwner, nil
		},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Register adds the HTTPHandler endpoints to the provided Echo router group.
func (h *HTTPHandler[S]) Register(g *echo.Group) {
	namespaces := g.Group("/namespaces")
	namespaces.POST("", h.CreateNamespace)
	namespaces.GET("", h.ListNamespaces)

	namespace := namespaces.Group(fmt.Sprintf("/:%s", PathParamNamespaceID))
	namespace.PUT("", h.UpdateNamespace)
	namespace.DELETE("", h.DeleteNamespace)
	namespace.POST("/operators", h.CreateOperator)

	operators := namespace.Group("/operators")
	operators.GET("", h.ListOperators)

	operator := operators.Group(fmt.Sprintf("/:%s", PathParamOperatorID))
	operator.GET("", h.GetOperator)
	operator.PUT("", h.UpdateOperator)
	operator.DELETE("", h.DeleteOperator)
	operator.GET("/nats-config", h.GetNATSConfig)
	operator.POST("/accounts", h.CreateAccount)
	operator.GET("/accounts", h.ListAccounts)

	account := namespace.Group(fmt.Sprintf("/accounts/:%s", PathParamAccountID))
	account.GET("", h.GetAccount)
	account.PUT("", h.UpdateAccount)
	account.DELETE("", h.DeleteAccount)
	account.POST("/users", h.CreateUser)
	account.GET("/users", h.ListUsers)

	user := namespace.Group(fmt.Sprintf("/users/:%s", PathParamUserID))
	user.GET("", h.GetUser)
	user.PUT("", h.UpdateUser)
	user.DELETE("", h.DeleteUser)
	user.GET("/creds", h.GetUserCreds)
	user.GET("/issuances", h.ListUserJWTIssuances)
}

// CreateNamespace handles POST requests to create a Namespace.
func (h *HTTPHandler[S]) CreateNamespace(c echo.Context) (err error) {
	ctx := c.Request().Context()

	req, err := server.BindRequest[CreateNamespaceRequest](c)
	if err != nil {
		return err
	}

	owner, err := h.ownerGetter(c)
	if err != nil {
		return err
	}

	ns := entity.NewNamespace(req.Name, owner)
	c.Set(logkey.NamespaceID, ns.ID)

	if err = h.store.CreateNamespace(ctx, ns); err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusCreated, NamespaceResponse{
		Namespace: *ns,
	})
}

// ListNamespaces handles GET requests to list Namespaces.
func (h *HTTPHandler[S]) ListNamespaces(c echo.Context) error {
	ctx := c.Request().Context()

	owner, err := h.ownerGetter(c)
	if err != nil {
		return err
	}

	namespaces, cursor, err := PaginateNamespaces(ctx, c, h.store, owner)
	if err != nil {
		return err
	}

	nsResps := make([]NamespaceResponse, len(namespaces))
	for i, ns := range namespaces {
		nsResps[i] = NamespaceResponse{
			Namespace: *ns,
		}
	}

	return server.SetResponseList(c, http.StatusOK, nsResps, cursor)
}

// UpdateNamespace handles PUT requests to update a Namespace.
func (h *HTTPHandler[S]) UpdateNamespace(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[UpdateNamespaceRequest](c)
	if err != nil {
		return err
	}

	nsID := id.MustParse[entity.NamespaceID](req.ID)
	c.Set(logkey.NamespaceID, nsID)

	ns, err := h.store.ReadNamespace(ctx, nsID)
	if err != nil {
		return err
	}

	ns.Name = req.Name

	if err = h.store.UpdateNamespace(ctx, ns); err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, NamespaceResponse{
		Namespace: *ns,
	})
}

// DeleteNamespace handles DELETE requests to delete a Namespace.
func (h *HTTPHandler[S]) DeleteNamespace(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[DeleteNamespaceRequest](c)
	if err != nil {
		return err
	}

	nsID := id.MustParse[entity.NamespaceID](req.ID)
	c.Set(logkey.NamespaceID, nsID)

	operatorCount, err := h.store.CountNamespaceOperators(ctx, nsID)
	if err != nil {
		return err
	}
	if operatorCount > 0 {
		return errtag.NewTagged[errtag.InvalidArgument](
			"cannot delete namespace with one or more operators",
			errtag.WithMsg("Namespace has one or more operators that must be deleted first"),
		)
	}

	if err = h.store.DeleteNamespace(ctx, nsID); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// CreateOperator handles POST requests to create an Operator.
func (h *HTTPHandler[S]) CreateOperator(c echo.Context) (err error) {
	ctx := c.Request().Context()

	req, err := server.BindRequest[CreateOperatorRequest](c)
	if err != nil {
		return err
	}
	nsID := id.MustParse[entity.NamespaceID](req.NamespaceID)

	op, err := entity.NewOperator(req.Name, nsID)
	if err != nil {
		return err
	}
	c.Set(logkey.OperatorID, op.ID)

	data, err := op.Data()
	if err != nil {
		return err
	}

	sysAcc, sysUser, err := op.SetNewSystemAccountAndUser()
	if err != nil {
		return err
	}
	c.Set(logkey.SystemAccountID, sysAcc.ID)
	c.Set(logkey.SystemUserID, sysUser.ID)

	err = h.store.BeginTxFunc(ctx, func(ctx context.Context, store S) error {
		if err = store.CreateOperator(ctx, op); err != nil {
			return err
		}
		if err = store.CreateAccount(ctx, sysAcc); err != nil {
			return err
		}
		return store.CreateUser(ctx, sysUser)
	})
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusCreated, OperatorResponse{
		OperatorData: data,
		Status: entity.OperatorNATSStatus{
			Connected: false,
		},
	})
}

// UpdateOperator handles PUT requests to update an Operator.
func (h *HTTPHandler[S]) UpdateOperator(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[UpdateOperatorRequest](c)
	if err != nil {
		return err
	}

	opID := id.MustParse[entity.OperatorID](req.ID)
	c.Set(logkey.OperatorID, opID)

	op, err := h.store.ReadOperator(ctx, opID)
	if err != nil {
		return err
	}

	if err = VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	if err = op.SetName(req.Name); err != nil {
		return err
	}

	if err = h.store.UpdateOperator(ctx, op); err != nil {
		return err
	}

	data, err := op.Data()
	if err != nil {
		return err
	}

	opStatus, err := h.commander.Ping(ctx, op.ID)
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, OperatorResponse{
		OperatorData: data,
		Status:       opStatus,
	})
}

// GetOperator handles GET requests to get an Operator.
func (h *HTTPHandler[S]) GetOperator(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetOperatorRequest](c)
	if err != nil {
		return err
	}

	opID := id.MustParse[entity.OperatorID](req.ID)
	c.Set(logkey.OperatorID, opID)

	op, err := h.store.ReadOperator(ctx, opID)
	if err != nil {
		return err
	}

	if err = VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	data, err := op.Data()
	if err != nil {
		return err
	}

	opStatus, err := h.commander.Ping(ctx, op.ID)
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, OperatorResponse{
		OperatorData: data,
		Status:       opStatus,
	})
}

// DeleteOperator handles DELETE requests to delete an Operator.
func (h *HTTPHandler[S]) DeleteOperator(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[DeleteOperatorRequest](c)
	if err != nil {
		return err
	}

	opID := id.MustParse[entity.OperatorID](req.ID)
	c.Set(logkey.OperatorID, opID)

	op, err := h.store.ReadOperator(ctx, opID)
	if err != nil {
		return err
	}

	if err = VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	if !req.UnmanageAccounts {
		accountCount, err := h.store.CountOperatorAccounts(ctx, opID)
		if err != nil {
			return err
		}
		if accountCount > 0 {
			return errtag.NewTagged[errtag.InvalidArgument](
				"cannot delete operator with one or more accounts",
				errtag.WithMsg("Operator has one or more accounts that must be deleted first"),
			)
		}
	}

	if err = h.store.DeleteOperator(ctx, opID); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// ListOperators handles GET requests to list Operators.
func (h *HTTPHandler[S]) ListOperators(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[ListOperatorsRequest](c)
	if err != nil {
		return err
	}

	nsID := id.MustParse[entity.NamespaceID](req.NamespaceID)
	c.Set(logkey.NamespaceID, nsID)

	ops, cursor, err := PaginateOperators(ctx, c, h.store, nsID)
	if err != nil {
		return err
	}

	errg := new(errgroup.Group)
	sem := make(chan struct{}, 10)

	opResps := make([]OperatorResponse, len(ops))
	for i, op := range ops {
		data, err := op.Data()
		if err != nil {
			return err
		}

		opResps[i] = OperatorResponse{
			OperatorData: data,
		}

		sem <- struct{}{}
		errg.Go(func() error {
			defer func() { <-sem }()
			opStatus, err := h.commander.Ping(ctx, op.ID)
			if err != nil {
				return err
			}
			opResps[i].Status = opStatus
			return nil
		})
	}

	if err = errg.Wait(); err != nil {
		return err
	}

	return server.SetResponseList(c, http.StatusOK, opResps, cursor)
}

// CreateAccount handles POST requests to create an Account.
// The associated Operator's NATS server will be notified of the new Account if
// a Commander has been configured.
func (h *HTTPHandler[S]) CreateAccount(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[CreateAccountRequest](c)
	if err != nil {
		return err
	}

	opID := id.MustParse[entity.OperatorID](req.OperatorID)
	c.Set(logkey.OperatorID, opID)

	op, err := h.store.ReadOperator(ctx, opID)
	if err != nil {
		return err
	}

	if err = VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	acc, err := entity.NewAccount(req.Name, op)
	if err != nil {
		return err
	}
	c.Set(logkey.AccountID, acc.ID)

	if req.Limits != nil {
		if err = acc.Update(op, toUpdateAccountParams(req.Name, req.Limits)); err != nil {
			return err
		}
	}

	accData, err := acc.Data()
	if err != nil {
		return err
	}

	claims, err := acc.Claims()
	if err != nil {
		return err
	}

	err = h.store.BeginTxFunc(ctx, func(ctx context.Context, store S) error {
		if err = store.CreateAccount(ctx, acc); err != nil {
			return err
		}
		if h.commander != nil {
			return h.commander.NotifyAccountClaimsUpdate(ctx, acc)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusCreated, AccountResponse{
		AccountData: accData,
		Limits:      LoadAccountLimits(accData, claims),
	})
}

// UpdateAccount handles PUT requests to update an Account.
// The associated Operator's NATS server will be notified of the Account update
// if a Commander has been configured.
func (h *HTTPHandler[S]) UpdateAccount(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[UpdateAccountRequest](c)
	if err != nil {
		return err
	}

	accID := id.MustParse[entity.AccountID](req.ID)
	c.Set(logkey.AccountID, accID)

	acc, err := h.store.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}
	c.Set(logkey.OperatorID, acc.OperatorID)

	if err = VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	if err = VerifyPublicAccount(acc); err != nil {
		return err
	}

	op, err := h.store.ReadOperator(ctx, acc.OperatorID)
	if err != nil {
		return err
	}

	if err = acc.Update(op, toUpdateAccountParams(req.Name, req.Limits)); err != nil {
		return err
	}

	accData, err := acc.Data()
	if err != nil {
		return err
	}

	claims, err := acc.Claims()
	if err != nil {
		return err
	}

	err = h.store.BeginTxFunc(ctx, func(ctx context.Context, store S) error {
		if err = store.UpdateAccount(ctx, acc); err != nil {
			return err
		}
		if h.commander != nil {
			return h.commander.NotifyAccountClaimsUpdate(ctx, acc)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, AccountResponse{
		AccountData: accData,
		Limits:      LoadAccountLimits(accData, claims),
	})
}

// GetAccount handles GET requests to get an Account.
func (h *HTTPHandler[S]) GetAccount(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetAccountRequest](c)
	if err != nil {
		return err
	}

	accID := id.MustParse[entity.AccountID](req.ID)
	c.Set(logkey.AccountID, accID)

	acc, err := h.store.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}
	c.Set(logkey.OperatorID, acc.OperatorID)

	if err = VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	if err = VerifyPublicAccount(acc); err != nil {
		return err
	}

	accData, err := acc.Data()
	if err != nil {
		return err
	}

	claims, err := acc.Claims()
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, AccountResponse{
		AccountData: accData,
		Limits:      LoadAccountLimits(accData, claims),
	})
}

// ListAccounts handles GET requests to list Accounts.
func (h *HTTPHandler[S]) ListAccounts(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[ListAccountsRequest](c)
	if err != nil {
		return err
	}

	opID := id.MustParse[entity.OperatorID](req.OperatorID)
	c.Set(logkey.OperatorID, opID)

	accs, cursor, err := PaginateAccounts(ctx, c, h.store, opID)
	if err != nil {
		return err
	}

	accResps := make([]AccountResponse, len(accs))
	for i, acc := range accs {
		accData, err := acc.Data()
		if err != nil {
			return err
		}

		if err = VerifyEntityNamespace(c, acc); err != nil {
			return err
		}

		claims, err := acc.Claims()
		if err != nil {
			return err
		}

		accResps[i] = AccountResponse{
			AccountData: accData,
			Limits:      LoadAccountLimits(accData, claims),
		}
	}

	return server.SetResponseList(c, http.StatusOK, accResps, cursor)
}

// DeleteAccount handles DELETE requests to delete an Account.
// The associated Operator's NATS server will be notified of the Account deletion
// if a Commander has been configured.
func (h *HTTPHandler[S]) DeleteAccount(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[DeleteAccountRequest](c)
	if err != nil {
		return err
	}

	accID := id.MustParse[entity.AccountID](req.ID)
	c.Set(logkey.AccountID, accID)

	acc, err := h.store.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}
	c.Set(logkey.OperatorID, acc.OperatorID)

	if err = VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	op, err := h.store.ReadOperator(ctx, acc.OperatorID)
	if err != nil {
		return err
	}

	if req.Unmanage {
		if err = h.store.DeleteAccount(ctx, accID); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	}

	err = h.store.BeginTxFunc(ctx, func(ctx context.Context, store S) error {
		if err = store.DeleteAccount(ctx, accID); err != nil {
			return err
		}
		if h.commander != nil {
			return h.commander.NotifyAccountClaimsDelete(ctx, op, acc)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// CreateUser handles POST requests to create a User.
func (h *HTTPHandler[S]) CreateUser(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[CreateUserRequest](c)
	if err != nil {
		return err
	}
	accID := id.MustParse[entity.AccountID](req.AccountID)
	c.Set(logkey.AccountID, accID)

	acc, err := h.store.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}
	c.Set(logkey.OperatorID, acc.OperatorID)

	if err = VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	if err = VerifyPublicAccount(acc); err != nil {
		return err
	}

	usr, err := entity.NewUser(req.Name, acc)
	if err != nil {
		return err
	}
	c.Set(logkey.UserID, usr.ID)

	if req.Limits != nil {
		if err = usr.Update(acc, toUpdateUserParams(req.Name, req.Limits)); err != nil {
			return err
		}
	}

	usrClaims, err := usr.Claims()
	if err != nil {
		return err
	}

	if err = h.store.CreateUser(ctx, usr); err != nil {
		return err
	}

	usrData, err := usr.Data()
	if err != nil {
		return err
	}

	pubKey, err := usr.NKey().KeyPair().PublicKey()
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusCreated, UserResponse{
		UserData:  usrData,
		PublicKey: pubKey,
		Limits:    LoadUserLimits(usrData, usrClaims),
	})
}

// UpdateUser handles PUT requests to update a User.
func (h *HTTPHandler[S]) UpdateUser(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[UpdateUserRequest](c)
	if err != nil {
		return err
	}

	userID := id.MustParse[entity.UserID](req.ID)
	c.Set(logkey.UserID, userID)

	usr, err := h.store.ReadUser(ctx, userID)
	if err != nil {
		return err
	}
	c.Set(logkey.OperatorID, usr.OperatorID)
	c.Set(logkey.AccountID, usr.AccountID)

	if err = VerifyEntityNamespace(c, usr); err != nil {
		return err
	}

	acc, err := h.store.ReadAccount(ctx, usr.AccountID)
	if err != nil {
		return err
	}

	if err = VerifyPublicAccount(acc); err != nil {
		return err
	}

	if err = usr.Update(acc, toUpdateUserParams(req.Name, req.Limits)); err != nil {
		return err
	}

	usrData, err := usr.Data()
	if err != nil {
		return err
	}

	usrClaims, err := usr.Claims()
	if err != nil {
		return err
	}

	var pubKey string
	err = h.store.BeginTxFunc(ctx, func(ctx context.Context, store S) error {
		if err = store.UpdateUser(ctx, usr); err != nil {
			return err
		}
		pubKey, err = usr.NKey().KeyPair().PublicKey()
		return err
	})
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, UserResponse{
		UserData:  usrData,
		PublicKey: pubKey,
		Limits:    LoadUserLimits(usrData, usrClaims),
	})
}

// GetUser handles GET requests to get a User.
func (h *HTTPHandler[S]) GetUser(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetUserRequest](c)
	if err != nil {
		return err
	}
	userID := id.MustParse[entity.UserID](req.ID)
	c.Set(logkey.UserID, userID)

	usr, err := h.store.ReadUser(ctx, userID)
	if err != nil {
		return err
	}
	c.Set(logkey.OperatorID, usr.OperatorID)
	c.Set(logkey.AccountID, usr.AccountID)

	if err = VerifyEntityNamespace(c, usr); err != nil {
		return err
	}

	if err = VerifyPublicUser(usr); err != nil {
		return err
	}

	usrClaims, err := usr.Claims()
	if err != nil {
		return err
	}

	usrData, err := usr.Data()
	if err != nil {
		return err
	}

	pubKey, err := usr.NKey().KeyPair().PublicKey()
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, UserResponse{
		UserData:  usrData,
		PublicKey: pubKey,
		Limits:    LoadUserLimits(usrData, usrClaims),
	})
}

// ListUsers handles GET requests to list users.
func (h *HTTPHandler[S]) ListUsers(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[ListUsersRequest](c)
	if err != nil {
		return err
	}

	accID := id.MustParse[entity.AccountID](req.AccountID)
	c.Set(logkey.AccountID, accID)

	acc, err := h.store.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}
	if err = VerifyPublicAccount(acc); err != nil {
		return err
	}

	users, cursor, err := PaginateUsers(ctx, c, h.store, accID)
	if err != nil {
		return err
	}

	userResps := make([]UserResponse, len(users))
	for i, usr := range users {
		if err = VerifyEntityNamespace(c, usr); err != nil {
			return err
		}

		usrData, err := usr.Data()
		if err != nil {
			return err
		}

		pubKey, err := usr.NKey().KeyPair().PublicKey()
		if err != nil {
			return err
		}

		usrClaims, err := usr.Claims()
		if err != nil {
			return err
		}

		userResps[i] = UserResponse{
			UserData:  usrData,
			PublicKey: pubKey,
			Limits:    LoadUserLimits(usrData, usrClaims),
		}
	}

	return server.SetResponseList(c, http.StatusOK, userResps, cursor)
}

// DeleteUser handles DELETE requests to delete a User.
func (h *HTTPHandler[S]) DeleteUser(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetUserRequest](c)
	if err != nil {
		return err
	}

	userID := id.MustParse[entity.UserID](req.ID)
	c.Set(logkey.UserID, userID)

	op, err := h.store.ReadUser(ctx, userID)
	if err != nil {
		return err
	}

	if err = VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	if err = h.store.DeleteUser(ctx, userID); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// GetUserCreds handles GET requests to get user credentials.
// Credentials are returned as a plaintext response in the structure below.
//
//	-----BEGIN NATS USER JWT-----
//	${USER_JWT}
//	------END NATS USER JWT------
//
//	************************* IMPORTANT *************************
//	NKEY Seed printed below can be used to sign and prove identity.
//	NKEYs are sensitive and should be treated as secrets.
//
//	-----BEGIN USER NKEY SEED-----
//	${NKEY_SEED}
//	------END USER NKEY SEED------
//
//	*************************************************************
func (h *HTTPHandler[S]) GetUserCreds(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetUserRequest](c)
	if err != nil {
		return err
	}
	userID := id.MustParse[entity.UserID](req.ID)
	c.Set(logkey.UserID, userID)

	usr, err := h.store.ReadUser(ctx, userID)
	if err != nil {
		return err
	}
	c.Set(logkey.OperatorID, usr.OperatorID)
	c.Set(logkey.AccountID, usr.AccountID)

	if err = VerifyEntityNamespace(c, usr); err != nil {
		return err
	}

	usrData, err := usr.Data()
	if err != nil {
		return err
	}

	acc, err := h.store.ReadAccount(ctx, usr.AccountID)
	if err != nil {
		return err
	}
	if err = VerifyPublicAccount(acc); err != nil {
		return err
	}

	accData, err := acc.Data()
	if err != nil {
		return err
	}

	refreshedJWT, _, err := usr.RefreshJWT(acc)
	if err != nil {
		return err
	}

	content, err := jwt.FormatUserConfig(refreshedJWT, usr.NKey().Seed)
	if err != nil {
		return err
	}

	if err = h.store.RecordUserJWTIssuance(ctx, usr); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-%s.creds", accData.Name, usrData.Name)
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextPlain)
	c.Response().Header().Set(echo.HeaderContentLength, strconv.Itoa(len(content)))
	c.Response().Header().Set(echo.HeaderAccessControlExposeHeaders, echo.HeaderContentDisposition)
	c.Response().Status = http.StatusOK
	return c.String(http.StatusOK, string(content))
}

func (h *HTTPHandler[S]) ListUserJWTIssuances(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetUserRequest](c)
	if err != nil {
		return err
	}
	userID := id.MustParse[entity.UserID](req.ID)
	c.Set(logkey.UserID, userID)

	usr, err := h.store.ReadUser(ctx, userID)
	if err != nil {
		return err
	}
	c.Set(logkey.OperatorID, usr.OperatorID)
	c.Set(logkey.AccountID, usr.AccountID)

	if err = VerifyEntityNamespace(c, usr); err != nil {
		return err
	}
	if err = VerifyPublicUser(usr); err != nil {
		return err
	}

	issuances, cursor, err := PaginateUserJWTIssuances(ctx, c, h.store, usr.ID)
	if err != nil {
		return err
	}

	resps := make([]UserJWTIssuanceResponse, len(issuances))
	for i, iss := range issuances {
		resps[i] = UserJWTIssuanceResponse{
			UserJWTIssuance: iss,
			Active:          iss.IsActive(),
		}
	}

	return server.SetResponseList(c, http.StatusOK, resps, cursor)
}

// GetNATSConfig handles GET requests to get a NATS server configuration file.
// for an Operator
func (h *HTTPHandler[S]) GetNATSConfig(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetOperatorRequest](c)
	if err != nil {
		return err
	}
	opID := id.MustParse[entity.OperatorID](req.ID)
	c.Set(logkey.OperatorID, opID)

	op, err := h.store.ReadOperator(ctx, opID)
	if err != nil {
		return err
	}

	if err = VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	opClaims, err := op.Claims()
	if err != nil {
		return err
	}

	if opClaims.SystemAccount == "" {
		return errors.New("operator has no system account")
	}

	sysAcc, err := h.store.ReadAccountByPublicKey(ctx, opClaims.SystemAccount)
	if err != nil {
		return err
	}
	c.Set(logkey.SystemAccountID, sysAcc.ID)

	content, err := entity.NewDirResolverConfig(op, sysAcc, entity.DefaultResolverDir)
	if err != nil {
		return err
	}

	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextPlain)
	c.Response().Header().Set(echo.HeaderContentLength, strconv.Itoa(len(content)))
	c.Response().Status = http.StatusOK
	return c.String(http.StatusOK, content)
}

func LoadUserLimits(userData entity.UserData, claims *jwt.UserClaims) UserLimits {
	limits := UserLimits{
		Subscriptions: parseLimit(claims.Subs),
		PayloadSize:   parseLimit(claims.Limits.Payload),
	}
	if userData.JWTDuration != nil {
		limits.JWTDurationSecs = ref.Ptr(int64(userData.JWTDuration.Seconds()))
	}
	return limits
}

func toUpdateAccountParams(name string, limits *AccountLimits) entity.UpdateAccountParams {
	params := entity.UpdateAccountParams{
		Name:          name,
		Subscriptions: ref.Deref(limits.Subscriptions, noLimit),
		PayloadSize:   ref.Deref(limits.PayloadSize, noLimit),
		Imports:       ref.Deref(limits.Imports, noLimit),
		Exports:       ref.Deref(limits.Exports, noLimit),
		Connections:   ref.Deref(limits.Connections, noLimit),
	}
	if limits.UserJWTDurationSecs != nil {
		params.UserJWTDuration = ref.Ptr(time.Duration(*limits.UserJWTDurationSecs) * time.Second)
	}
	return params
}

func LoadAccountLimits(accData entity.AccountData, claims *jwt.AccountClaims) AccountLimits {
	limits := AccountLimits{
		Subscriptions: parseLimit(claims.Limits.Subs),
		PayloadSize:   parseLimit(claims.Limits.Payload),
		Imports:       parseLimit(claims.Limits.Imports),
		Exports:       parseLimit(claims.Limits.Exports),
		Connections:   parseLimit(claims.Limits.Conn),
	}
	if accData.UserJWTDuration != nil {
		limits.UserJWTDurationSecs = ref.Ptr(int64(accData.UserJWTDuration.Seconds()))
	}
	return limits
}

func toUpdateUserParams(name string, limits *UserLimits) entity.UpdateUserParams {
	params := entity.UpdateUserParams{
		Name:          name,
		Subscriptions: ref.Deref(limits.Subscriptions, noLimit),
		PayloadSize:   ref.Deref(limits.PayloadSize, noLimit),
	}
	if limits.JWTDurationSecs != nil {
		params.JWTDuration = ref.Ptr(time.Duration(*limits.JWTDurationSecs) * time.Second)
	}
	return params
}

func parseLimit(l int64) *int64 {
	if l == -1 {
		return nil // no limit
	}
	return &l
}
