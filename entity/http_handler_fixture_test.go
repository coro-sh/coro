package entity_test // avoid import cycle with sqlite package

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/log"
	"github.com/coro-sh/coro/server"
	"github.com/coro-sh/coro/sqlite"
	"github.com/coro-sh/coro/testutil"
)

const (
	testTimeout                = 5 * time.Second
	stubNotifConnectTime int64 = 1738931738
)

type HTTPHandlerTestFixture struct {
	Server *server.Server
	Store  *entity.Store
	t      *testing.T
	stop   func()
}

func NewHTTPHandlerTestFixture(t *testing.T) *HTTPHandlerTestFixture {
	t.Helper()

	store, txer := sqlite.NewTestEntityStore(t)

	logger := log.NewLogger(log.WithDevelopment())
	srv, err := server.NewServer(testutil.GetFreePort(t),
		server.WithLogger(logger),
		server.WithMiddleware(entity.NamespaceContextMiddleware()),
	)
	require.NoError(t, err)
	srv.Register("", entity.NewHTTPHandler(txer, store, entity.WithCommander(new(commanderStub))))

	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)

	return &HTTPHandlerTestFixture{
		Server: srv,
		Store:  store,
		t:      t,
		stop: func() {
			srv.Stop(context.Background())
		},
	}
}

func (t *HTTPHandlerTestFixture) NamespacesURL() string {
	return t.URLForPath("/namespaces")
}

func (t *HTTPHandlerTestFixture) NamespaceURL(namespaceID entity.NamespaceID) string {
	return fmt.Sprintf("%s/%s", t.NamespacesURL(), namespaceID)
}

func (t *HTTPHandlerTestFixture) OperatorsURL(namespaceID entity.NamespaceID) string {
	return t.NamespaceURL(namespaceID) + "/operators"
}

func (t *HTTPHandlerTestFixture) OperatorURL(namespaceID entity.NamespaceID, operatorID entity.OperatorID) string {
	return fmt.Sprintf("%s/%s", t.OperatorsURL(namespaceID), operatorID)
}

func (t *HTTPHandlerTestFixture) OperatorNATSConfigURL(namespaceID entity.NamespaceID, operatorID entity.OperatorID) string {
	return fmt.Sprintf("%s/nats-config", t.OperatorURL(namespaceID, operatorID))
}

func (t *HTTPHandlerTestFixture) OperatorAccountsURL(namespaceID entity.NamespaceID, operatorID entity.OperatorID) string {
	return fmt.Sprintf("%s/accounts", t.OperatorURL(namespaceID, operatorID))
}

func (t *HTTPHandlerTestFixture) AccountsURL(namespaceID entity.NamespaceID, operatorID entity.OperatorID) string {
	return fmt.Sprintf("%s/accounts", t.OperatorURL(namespaceID, operatorID))
}

func (t *HTTPHandlerTestFixture) AccountURL(namespaceID entity.NamespaceID, accountID entity.AccountID) string {
	return fmt.Sprintf("%s/accounts/%s", t.NamespaceURL(namespaceID), accountID)
}

func (t *HTTPHandlerTestFixture) AccountUsersURL(namespaceID entity.NamespaceID, accountID entity.AccountID) string {
	return fmt.Sprintf("%s/users", t.AccountURL(namespaceID, accountID))
}

func (t *HTTPHandlerTestFixture) UsersURL(namespaceID entity.NamespaceID, accountID entity.AccountID) string {
	return fmt.Sprintf("%s/users", t.AccountURL(namespaceID, accountID))
}

func (t *HTTPHandlerTestFixture) UserURL(namespaceID entity.NamespaceID, userID entity.UserID) string {
	return fmt.Sprintf("%s/users/%s", t.NamespaceURL(namespaceID), userID)
}

func (t *HTTPHandlerTestFixture) UserCredsURL(namespaceID entity.NamespaceID, userID entity.UserID) string {
	return fmt.Sprintf("%s/creds", t.UserURL(namespaceID, userID))
}

func (t *HTTPHandlerTestFixture) OperatorJwtURL(operatorPubKey string) string {
	return t.URLForPath(fmt.Sprintf("/jwt/operators/%s", operatorPubKey))
}

func (t *HTTPHandlerTestFixture) AccountJwtURL(operatorPubKey string, accountPubKey string) string {
	return fmt.Sprintf("%s/accounts/%s", t.OperatorJwtURL(operatorPubKey), accountPubKey)
}

func (t *HTTPHandlerTestFixture) AccountsJwtURL(operatorPubKey string) string {
	return fmt.Sprintf("%s/accounts", t.OperatorJwtURL(operatorPubKey))
}

func (t *HTTPHandlerTestFixture) URLForPath(path string) string {
	path = "/" + strings.TrimPrefix(path, "/")
	return t.Server.Address() + path
}

func (t *HTTPHandlerTestFixture) AddNamespace(ctx context.Context) *entity.Namespace {
	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	require.NoError(t.t, t.Store.CreateNamespace(ctx, ns))
	return ns
}

func (t *HTTPHandlerTestFixture) AddOperator(ctx context.Context) *entity.Operator {
	op, err := entity.NewOperator(testutil.RandName(), t.AddNamespace(ctx).ID)
	require.NoError(t.t, err)
	require.NoError(t.t, t.Store.CreateOperator(ctx, op))
	return op
}

func (t *HTTPHandlerTestFixture) AddAccount(ctx context.Context) *entity.Account {
	acc, err := entity.NewAccount(testutil.RandName(), t.AddOperator(ctx))
	require.NoError(t.t, err)
	require.NoError(t.t, t.Store.CreateAccount(ctx, acc))
	return acc
}

func (t *HTTPHandlerTestFixture) AddUser(ctx context.Context) *entity.User {
	usr, err := entity.NewUser(testutil.RandName(), t.AddAccount(ctx))
	require.NoError(t.t, err)
	require.NoError(t.t, t.Store.CreateUser(ctx, usr))
	return usr
}

func (t *HTTPHandlerTestFixture) Stop() {
	t.stop()
}

type commanderStub struct{}

func (n *commanderStub) NotifyAccountClaimsUpdate(_ context.Context, _ *entity.Account) error {
	return nil
}

func (n *commanderStub) Ping(_ context.Context, _ entity.OperatorID) (entity.OperatorNATSStatus, error) {
	connectTime := stubNotifConnectTime
	return entity.OperatorNATSStatus{
		Connected:   true,
		ConnectTime: &connectTime,
	}, nil
}
