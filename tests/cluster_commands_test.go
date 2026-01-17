package tests

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/joshjon/kit/id"
	"github.com/joshjon/kit/log"
	"github.com/joshjon/kit/ref"
	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/testutil"
	"github.com/nats-io/jwt/v2"
	natsrv "github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/command"
	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/embedns"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/entityapi"
	"github.com/coro-sh/coro/proxyapi"
	"github.com/coro-sh/coro/sqlite"
	"github.com/coro-sh/coro/tkn"
)

const testTimeout = 5 * time.Second

func TestClusteredNotifications(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	db := sqlite.NewTestDB(t)
	repo := sqlite.NewEntityRepository(db)
	store := entity.NewStore(repo)

	tknRW := sqlite.NewOperatorTokenReadWriter(db)
	tknIssuer := tkn.NewOperatorIssuer(tknRW, tkn.OperatorTokenTypeProxy)

	namespace := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	err := store.CreateNamespace(ctx, namespace)
	require.NoError(t, err)

	// Add operator, sys account, and sys user to the store
	op, sysAcc := setupNewTestSysOperator(ctx, t, store, namespace)
	opClaims, err := op.Claims()
	require.NoError(t, err)
	// Add operator proxy token to the store
	proxyTkn, err := tknIssuer.Generate(ctx, op.ID)
	require.NoError(t, err)

	bOp, err := entity.NewOperator("broker_test_operator", id.New[entity.NamespaceID]())
	require.NoError(t, err)

	bSysAcc, bSysUsr, err := bOp.SetNewSystemAccountAndUser()
	require.NoError(t, err)

	startNewServer := func(nodeName string, clusterAddr string, routes string) (*server.Server, *natsrv.Server) {
		natsCfg := embedns.EmbeddedNATSConfig{
			Resolver: embedns.ResolverConfig{
				Operator:      bOp,
				SystemAccount: bSysAcc,
			},
			NodeName: nodeName,
			Cluster: &embedns.ClusterConfig{
				ClusterName:     "test_cluster",
				ClusterHostPort: clusterAddr,
				Routes:          natsrv.RoutesFromStr(routes),
			},
		}
		brokerNats, err := embedns.NewEmbeddedNATS(natsCfg)
		require.NoError(t, err)
		brokerNats.Start()
		require.True(t, brokerNats.ReadyForConnections(2*time.Second))

		srv := SetupTestServer(t, store, tknIssuer, brokerNats, bSysUsr)
		require.NoError(t, err)
		return srv, brokerNats
	}

	// Start 2 servers
	node1ClusterAddr := testutil.GetFreeHostPort(t)
	node2ClusterAddr := testutil.GetFreeHostPort(t)

	srv1, nats1 := startNewServer("test_node_1", node1ClusterAddr, "nats://"+node2ClusterAddr)
	defer nats1.Shutdown()
	defer srv1.Stop(ctx)

	srv2, nats2 := startNewServer("test_node_2", node2ClusterAddr, "nats://"+node1ClusterAddr)
	defer nats2.Shutdown()
	defer srv2.Stop(ctx)

	// Can take a bit of extra time for the cluster to be formed
	time.Sleep(150 * time.Millisecond)

	// Create a 'user facing' NATS server
	ns := newTestNATS(t, op, sysAcc)
	defer ns.Shutdown()
	// Create a proxy between the 'user facing' NATS server and broker 1
	brokerAddr1 := srv1.WebsSocketAddress() + "/broker"
	logger := log.NewLogger(log.WithDevelopment())
	pxy, err := command.NewProxy(ctx, ns.ClientURL(), brokerAddr1, proxyTkn, command.WithProxyLogger(logger))
	require.NoError(t, err)
	pxy.Start(ctx)
	defer pxy.Stop()

	// Create a new account via server 1
	req := entityapi.CreateAccountRequest{Name: testutil.RandName()}
	accSrv1URL := fmt.Sprintf("%s/namespaces/%s/operators/%s/accounts", srv1.Address(), namespace.ID, op.ID)
	createRes := testutil.Post[server.Response[entityapi.AccountResponse]](t, accSrv1URL, req)
	gotCreated := createRes.Data
	assert.False(t, gotCreated.ID.IsZero())

	// Check 'user facing' NATS server added the new account
	time.Sleep(20 * time.Millisecond) // dir acc resolver refreshes every 10ms
	gotNatsAcc, err := ns.LookupAccount(gotCreated.PublicKey)
	require.NoError(t, err)
	assert.Equal(t, gotNatsAcc.Name, gotCreated.PublicKey)
	assert.True(t, opClaims.SigningKeys.Contains(gotNatsAcc.Issuer))

	// Update the account via server 2
	accSrv2URL := fmt.Sprintf("%s/namespaces/%s/accounts/%s", srv2.Address(), namespace.ID, gotCreated.ID)
	updateReq := entityapi.UpdateAccountRequest{
		Name:   gotCreated.Name,
		Limits: &entityapi.AccountLimits{Subscriptions: ref.Ptr(rand.Int64N(100))},
	}
	updateRes := testutil.Put[server.Response[entityapi.AccountResponse]](t, accSrv2URL, updateReq)
	gotUpdated := updateRes.Data
	assert.Equal(t, gotUpdated.Limits, *updateReq.Limits)

	// Check NATS server updated the account subscriptions limit
	time.Sleep(20 * time.Millisecond) // dir acc resolver refreshes every 10ms
	gotNatsAccJWT, err := ns.AccountResolver().Fetch(gotUpdated.PublicKey)
	require.NoError(t, err)
	claims, err := jwt.DecodeAccountClaims(gotNatsAccJWT)
	require.NoError(t, err)
	assert.Equal(t, *updateReq.Limits.Subscriptions, claims.Limits.Subs)
}

func setupNewTestSysOperator(ctx context.Context, t *testing.T, testStore *entity.Store, ns *entity.Namespace) (*entity.Operator, *entity.Account) {
	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	require.NoError(t, err)
	sysAcc, sysUser, err := op.SetNewSystemAccountAndUser()
	require.NoError(t, err)
	err = testStore.CreateOperator(ctx, op)
	require.NoError(t, err)
	err = testStore.CreateAccount(ctx, sysAcc)
	require.NoError(t, err)
	err = testStore.CreateUser(ctx, sysUser)
	require.NoError(t, err)
	return op, sysAcc
}

func SetupTestServer(
	t *testing.T,
	store *entity.Store,
	tknIssuer *tkn.OperatorIssuer,
	brokerNats *natsrv.Server,
	bSysUser *entity.User,
) *server.Server {
	t.Helper()

	logger := log.NewLogger(log.WithDevelopment())

	srv, err := server.NewServer(testutil.GetFreePort(t),
		server.WithLogger(logger),
		server.WithMiddleware(entityapi.NamespaceContextMiddleware()),
	)
	require.NoError(t, err)

	commander, err := command.NewCommander("", bSysUser, command.WithCommanderEmbeddedNATS(brokerNats))
	require.NoError(t, err)

	brokerHandler, err := command.NewBrokerWebSocketHandler(bSysUser, brokerNats, tknIssuer, store, command.WithBrokerWebsocketLogger(logger))
	require.NoError(t, err)

	srv.Register("", entityapi.NewHTTPHandler(store, entityapi.WithCommander[*entity.Store](commander)))
	srv.Register("", proxyapi.NewProxyHTTPHandler(tknIssuer, store, commander))
	srv.Register("", brokerHandler)

	go srv.Start()
	err = srv.WaitHealthy(10, time.Millisecond)
	require.NoError(t, err)

	return srv
}

func newTestNATS(t *testing.T, op *entity.Operator, sysAcc *entity.Account) *natsrv.Server {
	t.Helper()
	cfgContent, err := entity.NewDirResolverConfig(op, sysAcc, t.TempDir())
	require.NoError(t, err)

	cfgFile, err := os.CreateTemp(t.TempDir(), "")
	require.NoError(t, err)
	defer cfgFile.Close()

	err = os.WriteFile(cfgFile.Name(), []byte(cfgContent), 0666)
	require.NoError(t, err)

	opts, err := natsrv.ProcessConfigFile(cfgFile.Name())
	require.NoError(t, err)

	opts.Port = natsrv.RANDOM_PORT

	ns, err := natsrv.NewServer(opts)
	require.NoError(t, err)
	ns.Start()

	require.True(t, ns.ReadyForConnections(testTimeout))
	return ns
}
