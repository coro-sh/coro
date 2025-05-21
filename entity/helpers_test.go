package entity

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/encrypt"
	"github.com/coro-sh/coro/internal/testutil"
	"github.com/coro-sh/coro/log"
	"github.com/coro-sh/coro/server"
)

const (
	testTimeout                = 5 * time.Second
	apiPrefix                  = "/api/v1"
	stubNotifConnectTime int64 = 1738931738
)

type TestFixture struct {
	Server *server.Server
	Store  *Store
	stop   func()
}

func SetupTestFixture(t *testing.T) *TestFixture {
	t.Helper()

	aesEnc, err := encrypt.NewAES(testutil.RandString(32))
	require.NoError(t, err)
	txer := new(testutil.FakeTxer)
	store := NewStore(txer, NewFakeEntityRepository(t), WithEncryption(aesEnc))

	logger := log.NewLogger(log.WithDevelopment())
	srv, err := server.NewServer(testutil.GetFreePort(t),
		server.WithLogger(logger),
		server.WithMiddleware(NamespaceContextMiddleware()),
	)
	require.NoError(t, err)
	srv.Register(NewHTTPHandler(txer, store, WithCommander(new(commanderStub))))

	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)

	return &TestFixture{
		Server: srv,
		Store:  store,
		stop: func() {
			srv.Stop(context.Background())
		},
	}
}

func (t *TestFixture) NamespacesURL() string {
	return t.URLForPath(fmt.Sprintf("%s/namespaces", apiPrefix))
}

func (t *TestFixture) NamespaceURL(namespaceID NamespaceID) string {
	return fmt.Sprintf("%s/%s", t.NamespacesURL(), namespaceID)
}

func (t *TestFixture) OperatorsURL(namespaceID NamespaceID) string {
	return t.NamespaceURL(namespaceID) + "/operators"
}

func (t *TestFixture) OperatorURL(namespaceID NamespaceID, operatorID OperatorID) string {
	return fmt.Sprintf("%s/%s", t.OperatorsURL(namespaceID), operatorID)
}

func (t *TestFixture) OperatorNATSConfigURL(namespaceID NamespaceID, operatorID OperatorID) string {
	return fmt.Sprintf("%s/nats-config", t.OperatorURL(namespaceID, operatorID))
}

func (t *TestFixture) OperatorAccountsURL(namespaceID NamespaceID, operatorID OperatorID) string {
	return fmt.Sprintf("%s/accounts", t.OperatorURL(namespaceID, operatorID))
}

func (t *TestFixture) AccountsURL(namespaceID NamespaceID, operatorID OperatorID) string {
	return fmt.Sprintf("%s/accounts", t.OperatorURL(namespaceID, operatorID))
}

func (t *TestFixture) AccountURL(namespaceID NamespaceID, accountID AccountID) string {
	return fmt.Sprintf("%s/accounts/%s", t.NamespaceURL(namespaceID), accountID)
}

func (t *TestFixture) AccountUsersURL(namespaceID NamespaceID, accountID AccountID) string {
	return fmt.Sprintf("%s/users", t.AccountURL(namespaceID, accountID))
}

func (t *TestFixture) UsersURL(namespaceID NamespaceID, accountID AccountID) string {
	return fmt.Sprintf("%s/users", t.AccountURL(namespaceID, accountID))
}

func (t *TestFixture) UserURL(namespaceID NamespaceID, userID UserID) string {
	return fmt.Sprintf("%s/users/%s", t.NamespaceURL(namespaceID), userID)
}

func (t *TestFixture) UserCredsURL(namespaceID NamespaceID, userID UserID) string {
	return fmt.Sprintf("%s/creds", t.UserURL(namespaceID, userID))
}

func (t *TestFixture) OperatorJwtURL(operatorPubKey string) string {
	return t.URLForPath(fmt.Sprintf("%s/jwt/operators/%s", apiPrefix, operatorPubKey))
}

func (t *TestFixture) AccountJwtURL(operatorPubKey string, accountPubKey string) string {
	return fmt.Sprintf("%s/accounts/%s", t.OperatorJwtURL(operatorPubKey), accountPubKey)
}

func (t *TestFixture) AccountsJwtURL(operatorPubKey string) string {
	return fmt.Sprintf("%s/accounts", t.OperatorJwtURL(operatorPubKey))
}

func (t *TestFixture) URLForPath(path string) string {
	path = "/" + strings.TrimPrefix(path, "/")
	return t.Server.Address() + path
}

func (t *TestFixture) Stop() {
	t.stop()
}

type commanderStub struct{}

func (n *commanderStub) NotifyAccountClaimsUpdate(_ context.Context, _ *Account) error {
	return nil
}

func (n *commanderStub) Ping(_ context.Context, _ OperatorID) (OperatorNATSStatus, error) {
	connectTime := stubNotifConnectTime
	return OperatorNATSStatus{
		Connected:   true,
		ConnectTime: &connectTime,
	}, nil
}
