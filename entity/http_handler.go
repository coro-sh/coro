package entity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nats-io/jwt/v2"
	"golang.org/x/sync/errgroup"

	"github.com/coro-sh/coro/log"
	"github.com/coro-sh/coro/server"
	"github.com/coro-sh/coro/tx"
)

const (
	VersionPath          = "/v1"
	PathParamNamespaceID = "namespace_id"
	PathParamOperatorID  = "operator_id"
	PathParamAccountID   = "account_id"
	PathParamUserID      = "user_id"
)

// Commander sends commands to a NATS server.
type Commander interface {
	NotifyAccountClaimsUpdate(ctx context.Context, account *Account) error
	// Ping checks if an Operator is connected and ready to receive commands.
	Ping(ctx context.Context, operatorID OperatorID) (OperatorNATSStatus, error)
}

// HTTPHandlerOption configures a HTTPHandler.
type HTTPHandlerOption func(handler *HTTPHandler)

// WithCommander sets a Commander on the handler to enable commands.
func WithCommander(commander Commander) HTTPHandlerOption {
	return func(handler *HTTPHandler) {
		handler.commander = commander
	}
}

// HTTPHandler handles entity HTTP requests.
type HTTPHandler struct {
	txer      tx.Txer
	store     *Store
	commander Commander
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(txer tx.Txer, store *Store, opts ...HTTPHandlerOption) *HTTPHandler {
	h := &HTTPHandler{
		txer:  txer,
		store: store,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Register adds the HTTPHandler endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	v1 := g.Group(VersionPath)

	namespaces := v1.Group("/namespaces")
	namespaces.POST("", h.CreateNamespace)
	namespaces.GET("", h.ListNamespaces)

	namespace := namespaces.Group(fmt.Sprintf("/:%s", PathParamNamespaceID))
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
func (h *HTTPHandler) CreateNamespace(c echo.Context) (err error) {
	ctx := c.Request().Context()

	req, err := server.BindRequest[CreateNamespaceRequest](c)
	if err != nil {
		return err
	}

	ns := NewNamespace(req.Name)
	c.Set(log.KeyNamespaceID, ns.ID)

	if err = h.store.CreateNamespace(ctx, ns); err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusCreated, NamespaceResponse{
		Namespace: *ns,
	})
}

// ListNamespaces handles GET requests to list Namespaces.
func (h *HTTPHandler) ListNamespaces(c echo.Context) error {
	ctx := c.Request().Context()

	namespaces, cursor, err := PaginateNamespaces(ctx, c, h.store)
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

// DeleteNamespace handles DELETE requests to delete a Namespace.
func (h *HTTPHandler) DeleteNamespace(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[DeleteNamespaceRequest](c)
	if err != nil {
		return err
	}

	nsID := MustParseID[NamespaceID](req.ID)
	c.Set(log.KeyNamespaceID, nsID)

	if err = h.store.DeleteNamespace(ctx, nsID); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// CreateOperator handles POST requests to create an Operator.
func (h *HTTPHandler) CreateOperator(c echo.Context) (err error) {
	ctx := c.Request().Context()

	req, err := server.BindRequest[CreateOperatorRequest](c)
	if err != nil {
		return err
	}
	nsID := MustParseID[NamespaceID](req.NamespaceID)

	op, err := NewOperator(req.Name, nsID)
	if err != nil {
		return err
	}
	c.Set(log.KeyOperatorID, op.ID)

	data, err := op.Data()
	if err != nil {
		return err
	}

	sysAcc, sysUser, err := op.SetNewSystemAccountAndUser()
	if err != nil {
		return err
	}
	c.Set(log.KeySystemAccountID, sysAcc.ID)
	c.Set(log.KeySystemUserID, sysUser.ID)

	txn, err := h.txer.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Handle(ctx, txn, &err)

	store, err := h.store.WithTx(txn)
	if err != nil {
		return err
	}

	if err = store.CreateOperator(ctx, op); err != nil {
		return err
	}

	if err = store.CreateAccount(ctx, sysAcc); err != nil {
		return err
	}

	if err = store.CreateUser(ctx, sysUser); err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusCreated, OperatorResponse{
		OperatorData: data,
		Status: OperatorNATSStatus{
			Connected: false,
		},
	})
}

// UpdateOperator handles PUT requests to update an Operator.
func (h *HTTPHandler) UpdateOperator(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[UpdateOperatorRequest](c)
	if err != nil {
		return err
	}

	opID := MustParseID[OperatorID](req.ID)
	c.Set(log.KeyOperatorID, opID)

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
func (h *HTTPHandler) GetOperator(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetOperatorRequest](c)
	if err != nil {
		return err
	}

	opID := MustParseID[OperatorID](req.ID)
	c.Set(log.KeyOperatorID, opID)

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
func (h *HTTPHandler) DeleteOperator(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetOperatorRequest](c)
	if err != nil {
		return err
	}

	opID := MustParseID[OperatorID](req.ID)
	c.Set(log.KeyOperatorID, opID)

	op, err := h.store.ReadOperator(ctx, opID)
	if err != nil {
		return err
	}

	if err = VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	if err = h.store.DeleteOperator(ctx, opID); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// ListOperators handles GET requests to list Operators.
func (h *HTTPHandler) ListOperators(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[ListOperatorsRequest](c)
	if err != nil {
		return err
	}

	nsID := MustParseID[NamespaceID](req.NamespaceID)
	c.Set(log.KeyNamespaceID, nsID)

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
func (h *HTTPHandler) CreateAccount(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[CreateAccountRequest](c)
	if err != nil {
		return err
	}

	opID := MustParseID[OperatorID](req.OperatorID)
	c.Set(log.KeyOperatorID, opID)

	op, err := h.store.ReadOperator(ctx, opID)
	if err != nil {
		return err
	}

	if err = VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	acc, err := NewAccount(req.Name, op)
	if err != nil {
		return err
	}
	c.Set(log.KeyAccountID, acc.ID)

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

	txn, err := h.txer.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Handle(ctx, txn, &err)

	store, err := h.store.WithTx(txn)
	if err != nil {
		return err
	}

	if err = store.CreateAccount(ctx, acc); err != nil {
		return err
	}

	if h.commander != nil {
		if err = h.commander.NotifyAccountClaimsUpdate(ctx, acc); err != nil {
			return err
		}
	}

	return server.SetResponse(c, http.StatusCreated, AccountResponse{
		AccountData: accData,
		Limits:      loadAccountLimits(accData, claims),
	})
}

// UpdateAccount handles PUT requests to update an Account.
// The associated Operator's NATS server will be notified of the Account update
// if a Commander has been configured.
func (h *HTTPHandler) UpdateAccount(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[UpdateAccountRequest](c)
	if err != nil {
		return err
	}

	accID := MustParseID[AccountID](req.ID)
	c.Set(log.KeyAccountID, accID)

	acc, err := h.store.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}
	c.Set(log.KeyOperatorID, acc.OperatorID)

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

	txn, err := h.txer.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Handle(ctx, txn, &err)

	store, err := h.store.WithTx(txn)
	if err != nil {
		return err
	}

	if err = store.UpdateAccount(ctx, acc); err != nil {
		return err
	}

	if h.commander != nil {
		if err = h.commander.NotifyAccountClaimsUpdate(ctx, acc); err != nil {
			return err
		}
	}

	return server.SetResponse(c, http.StatusOK, AccountResponse{
		AccountData: accData,
		Limits:      loadAccountLimits(accData, claims),
	})
}

// GetAccount handles GET requests to get an Account.
func (h *HTTPHandler) GetAccount(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetAccountRequest](c)
	if err != nil {
		return err
	}

	accID := MustParseID[AccountID](req.ID)
	c.Set(log.KeyAccountID, accID)

	acc, err := h.store.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}
	c.Set(log.KeyOperatorID, acc.OperatorID)

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
		Limits:      loadAccountLimits(accData, claims),
	})
}

// ListAccounts handles GET requests to list Accounts.
func (h *HTTPHandler) ListAccounts(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[ListAccountsRequest](c)
	if err != nil {
		return err
	}

	opID := MustParseID[OperatorID](req.OperatorID)
	c.Set(log.KeyOperatorID, opID)

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
			Limits:      loadAccountLimits(accData, claims),
		}
	}

	return server.SetResponseList(c, http.StatusOK, accResps, cursor)
}

// DeleteAccount handles DELETE requests to delete an Account.
func (h *HTTPHandler) DeleteAccount(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetAccountRequest](c)
	if err != nil {
		return err
	}

	accID := MustParseID[AccountID](req.ID)
	c.Set(log.KeyAccountID, accID)

	op, err := h.store.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}

	if err = VerifyEntityNamespace(c, op); err != nil {
		return err
	}

	if err = h.store.DeleteAccount(ctx, accID); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// CreateUser handles POST requests to create a User.
func (h *HTTPHandler) CreateUser(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[CreateUserRequest](c)
	if err != nil {
		return err
	}
	accID := MustParseID[AccountID](req.AccountID)
	c.Set(log.KeyAccountID, accID)

	acc, err := h.store.ReadAccount(ctx, accID)
	if err != nil {
		return err
	}
	c.Set(log.KeyOperatorID, acc.OperatorID)

	if err = VerifyEntityNamespace(c, acc); err != nil {
		return err
	}

	if err = VerifyPublicAccount(acc); err != nil {
		return err
	}

	usr, err := NewUser(req.Name, acc)
	if err != nil {
		return err
	}
	c.Set(log.KeyUserID, usr.ID)

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
		Limits:    loadUserLimits(usrData, usrClaims),
	})
}

// UpdateUser handles PUT requests to update a User.
func (h *HTTPHandler) UpdateUser(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[UpdateUserRequest](c)
	if err != nil {
		return err
	}

	userID := MustParseID[UserID](req.ID)
	c.Set(log.KeyUserID, userID)

	usr, err := h.store.ReadUser(ctx, userID)
	if err != nil {
		return err
	}
	c.Set(log.KeyOperatorID, usr.OperatorID)
	c.Set(log.KeyAccountID, usr.AccountID)

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

	txn, err := h.txer.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Handle(ctx, txn, &err)

	store, err := h.store.WithTx(txn)
	if err != nil {
		return err
	}

	if err = store.UpdateUser(ctx, usr); err != nil {
		return err
	}

	pubKey, err := usr.NKey().KeyPair().PublicKey()
	if err != nil {
		return err
	}

	return server.SetResponse(c, http.StatusOK, UserResponse{
		UserData:  usrData,
		PublicKey: pubKey,
		Limits:    loadUserLimits(usrData, usrClaims),
	})
}

// GetUser handles GET requests to get a User.
func (h *HTTPHandler) GetUser(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetUserRequest](c)
	if err != nil {
		return err
	}
	userID := MustParseID[UserID](req.ID)
	c.Set(log.KeyUserID, userID)

	usr, err := h.store.ReadUser(ctx, userID)
	if err != nil {
		return err
	}
	c.Set(log.KeyOperatorID, usr.OperatorID)
	c.Set(log.KeyAccountID, usr.AccountID)

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
		Limits:    loadUserLimits(usrData, usrClaims),
	})
}

// ListUsers handles GET requests to list users.
func (h *HTTPHandler) ListUsers(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[ListUsersRequest](c)
	if err != nil {
		return err
	}

	accID := MustParseID[AccountID](req.AccountID)
	c.Set(log.KeyAccountID, accID)

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
			Limits:    loadUserLimits(usrData, usrClaims),
		}
	}

	return server.SetResponseList(c, http.StatusOK, userResps, cursor)
}

// DeleteUser handles DELETE requests to delete a User.
func (h *HTTPHandler) DeleteUser(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetUserRequest](c)
	if err != nil {
		return err
	}

	userID := MustParseID[UserID](req.ID)
	c.Set(log.KeyUserID, userID)

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
func (h *HTTPHandler) GetUserCreds(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetUserRequest](c)
	if err != nil {
		return err
	}
	userID := MustParseID[UserID](req.ID)
	c.Set(log.KeyUserID, userID)

	usr, err := h.store.ReadUser(ctx, userID)
	if err != nil {
		return err
	}
	c.Set(log.KeyOperatorID, usr.OperatorID)
	c.Set(log.KeyAccountID, usr.AccountID)

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

func (h *HTTPHandler) ListUserJWTIssuances(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetUserRequest](c)
	if err != nil {
		return err
	}
	userID := MustParseID[UserID](req.ID)
	c.Set(log.KeyUserID, userID)

	usr, err := h.store.ReadUser(ctx, userID)
	if err != nil {
		return err
	}
	c.Set(log.KeyOperatorID, usr.OperatorID)
	c.Set(log.KeyAccountID, usr.AccountID)

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
func (h *HTTPHandler) GetNATSConfig(c echo.Context) error {
	ctx := c.Request().Context()

	req, err := server.BindRequest[GetOperatorRequest](c)
	if err != nil {
		return err
	}
	opID := MustParseID[OperatorID](req.ID)
	c.Set(log.KeyOperatorID, opID)

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
	c.Set(log.KeySystemAccountID, sysAcc.ID)

	content, err := NewDirResolverConfig(op, sysAcc, defaultResolverDir)
	if err != nil {
		return err
	}

	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextPlain)
	c.Response().Header().Set(echo.HeaderContentLength, strconv.Itoa(len(content)))
	c.Response().Status = http.StatusOK
	return c.String(http.StatusOK, content)
}

func toUpdateAccountParams(name string, limits *AccountLimits) UpdateAccountParams {
	params := UpdateAccountParams{
		Name:          name,
		Subscriptions: deref(limits.Subscriptions, noLimit),
		PayloadSize:   deref(limits.PayloadSize, noLimit),
		Imports:       deref(limits.Imports, noLimit),
		Exports:       deref(limits.Exports, noLimit),
		Connections:   deref(limits.Connections, noLimit),
	}
	if limits.UserJWTDurationSecs != nil {
		params.UserJWTDuration = ptr(time.Duration(*limits.UserJWTDurationSecs) * time.Second)
	}
	return params
}

func loadAccountLimits(accData AccountData, claims *jwt.AccountClaims) AccountLimits {
	limits := AccountLimits{
		Subscriptions: parseLimit(claims.Limits.Subs),
		PayloadSize:   parseLimit(claims.Limits.Payload),
		Imports:       parseLimit(claims.Limits.Imports),
		Exports:       parseLimit(claims.Limits.Exports),
		Connections:   parseLimit(claims.Limits.Conn),
	}
	if accData.UserJWTDuration != nil {
		limits.UserJWTDurationSecs = ptr(int64(accData.UserJWTDuration.Seconds()))
	}
	return limits
}

func toUpdateUserParams(name string, limits *UserLimits) UpdateUserParams {
	params := UpdateUserParams{
		Name:          name,
		Subscriptions: deref(limits.Subscriptions, noLimit),
		PayloadSize:   deref(limits.PayloadSize, noLimit),
	}
	if limits.JWTDurationSecs != nil {
		params.JWTDuration = ptr(time.Duration(*limits.JWTDurationSecs) * time.Second)
	}
	return params
}

func loadUserLimits(userData UserData, claims *jwt.UserClaims) UserLimits {
	limits := UserLimits{
		Subscriptions: parseLimit(claims.Limits.Subs),
		PayloadSize:   parseLimit(claims.Limits.Payload),
	}
	if userData.JWTDuration != nil {
		limits.JWTDurationSecs = ptr(int64(userData.JWTDuration.Seconds()))
	}
	return limits
}

func ptr[T any](v T) *T {
	return &v
}

func deref[T any](ptr *T, defaultValue T) T {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

func parseLimit(l int64) *int64 {
	if l == -1 {
		return nil // no limit
	}
	return &l
}
