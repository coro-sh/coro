package entity

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	natsrv "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/internal/testutil"
	"github.com/coro-sh/coro/server"
)

func TestServer_CreateNamespace(t *testing.T) {
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	req := CreateNamespaceRequest{
		Name: testutil.RandName(),
	}

	res := testutil.Post[server.Response[NamespaceResponse]](t, fixture.NamespacesURL(), req)
	got := res.Data
	assert.False(t, got.ID.IsZero())
	assert.Equal(t, req.Name, got.Name)
}

func TestServer_DeleteNamespace(t *testing.T) {
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	ns := NewNamespace(testutil.RandName())
	err := fixture.Store.CreateNamespace(t.Context(), ns)
	require.NoError(t, err)

	testutil.Delete(t, fixture.NamespaceURL(ns.ID))

	_, err = fixture.Store.ReadNamespaceByName(t.Context(), ns.Name)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestHTTPHandler_ListNamespaces(t *testing.T) {
	ctx := context.Background()
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	numNS := 15
	pageSize := 10

	wantResps := make([]NamespaceResponse, numNS)

	for i := range numNS {
		ns := NewNamespace(testutil.RandName())
		err := fixture.Store.CreateNamespace(ctx, ns)
		require.NoError(t, err)

		wantResps[i] = NamespaceResponse{
			Namespace: *ns,
		}
	}

	// Page 1
	url := fmt.Sprintf("%s?%s=%d", fixture.NamespacesURL(), pageSizeQueryParam, pageSize)
	res := testutil.Get[server.ResponseList[NamespaceResponse]](t, url)
	assert.Equal(t, wantResps[:pageSize], res.Data)
	assert.NotEmpty(t, res.NextPageCursor)

	// Page 2
	url = fmt.Sprintf("%s&%s=%s", url, pageCursorQueryParam, *res.NextPageCursor)
	res = testutil.Get[server.ResponseList[NamespaceResponse]](t, url)
	assert.Equal(t, wantResps[pageSize:], res.Data)
	assert.Nil(t, res.NextPageCursor)
}

func TestServer_CreateOperator(t *testing.T) {
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	req := CreateOperatorRequest{
		Name: testutil.RandName(),
	}

	res := testutil.Post[server.Response[OperatorResponse]](t, fixture.OperatorsURL(NewID[NamespaceID]()), req)
	got := res.Data
	assert.False(t, got.ID.IsZero())
	assert.NotEmpty(t, got.JWT)
	assert.Equal(t, req.Name, got.Name)
	assert.NotEmpty(t, got.PublicKey)
}

func TestServer_UpdateOperator(t *testing.T) {
	ctx := context.Background()
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	err = fixture.Store.CreateOperator(ctx, op)
	require.NoError(t, err)

	req := UpdateOperatorRequest{
		Name: testutil.RandName(),
	}

	res := testutil.Put[server.Response[OperatorResponse]](t, fixture.OperatorURL(op.NamespaceID, op.ID), req)
	got := res.Data

	opData, err := op.Data()
	require.NoError(t, err)
	assert.Equal(t, opData.ID, got.ID)
	assert.Equal(t, opData.PublicKey, got.PublicKey)

	// Updated fields
	assert.NotEmpty(t, got.JWT)
	assert.NotEqual(t, opData.JWT, got.JWT)
	assert.Equal(t, req.Name, got.Name)
}

func TestServer_GetOperator(t *testing.T) {
	ctx := context.Background()
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	err = fixture.Store.CreateOperator(ctx, op)
	require.NoError(t, err)

	res := testutil.Get[server.Response[OperatorResponse]](t, fixture.OperatorURL(op.NamespaceID, op.ID))
	got := res.Data
	opData, err := op.Data()
	require.NoError(t, err)
	assert.Equal(t, opData, got.OperatorData)
}

func TestHTTPHandler_ListOperators(t *testing.T) {
	ctx := context.Background()
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	numOps := 15
	pageSize := 10

	nsID := NewID[NamespaceID]()
	wantResps := make([]OperatorResponse, numOps)

	for i := range numOps {
		op, err := NewOperator(testutil.RandName(), nsID)
		require.NoError(t, err)
		err = fixture.Store.CreateOperator(ctx, op)
		require.NoError(t, err)

		wantData, err := op.Data()
		require.NoError(t, err)

		wantResps[i] = OperatorResponse{
			OperatorData: wantData,
			Status: OperatorNATSStatus{
				Connected:   true,
				ConnectTime: ptr(stubNotifConnectTime),
			},
		}
	}

	// Page 1
	url := fmt.Sprintf("%s?%s=%d", fixture.OperatorsURL(nsID), pageSizeQueryParam, pageSize)
	res := testutil.Get[server.ResponseList[OperatorResponse]](t, url)
	assert.Equal(t, wantResps[:pageSize], res.Data)
	assert.NotEmpty(t, res.NextPageCursor)

	// Page 2
	url = fmt.Sprintf("%s&%s=%s", url, pageCursorQueryParam, *res.NextPageCursor)
	res = testutil.Get[server.ResponseList[OperatorResponse]](t, url)
	assert.Equal(t, wantResps[pageSize:], res.Data)
	assert.Nil(t, res.NextPageCursor)
}

func TestServer_DeleteOperator(t *testing.T) {
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	nsID := NewID[NamespaceID]()
	op, err := NewOperator(testutil.RandName(), nsID)
	require.NoError(t, err)

	err = fixture.Store.CreateOperator(t.Context(), op)
	require.NoError(t, err)

	testutil.Delete(t, fixture.OperatorURL(nsID, op.ID))

	_, err = fixture.Store.ReadOperator(t.Context(), op.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestServer_CreateAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	err = fixture.Store.CreateOperator(ctx, op)
	require.NoError(t, err)

	req := CreateAccountRequest{
		Name: testutil.RandName(),
		Limits: &AccountLimits{
			Subscriptions:       ptr(rand.Int64N(100)),
			PayloadSize:         ptr(rand.Int64N(100)),
			Imports:             ptr(rand.Int64N(100)),
			Exports:             ptr(rand.Int64N(100)),
			Connections:         ptr(rand.Int64N(100)),
			UserJWTDurationSecs: ptr(rand.Int64N(100000)),
		},
	}

	res := testutil.Post[server.Response[AccountResponse]](t, fixture.OperatorAccountsURL(op.NamespaceID, op.ID), req)
	got := res.Data

	assert.False(t, got.ID.IsZero())
	assert.NotEmpty(t, got.JWT)
	assert.Equal(t, op.ID, got.OperatorID)
	assert.Equal(t, req.Name, got.Name)
	assert.NotEmpty(t, got.PublicKey)
	assert.Equal(t, *req.Limits, got.Limits)
}

func TestServer_UpdateAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	err = fixture.Store.CreateOperator(ctx, op)
	require.NoError(t, err)

	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	err = fixture.Store.CreateAccount(ctx, acc)
	require.NoError(t, err)

	req := UpdateAccountRequest{
		Name: testutil.RandName(),
		Limits: &AccountLimits{
			Subscriptions: ptr(rand.Int64N(100)),
			PayloadSize:   ptr(rand.Int64N(100)),
			Imports:       ptr(rand.Int64N(100)),
			Exports:       ptr(rand.Int64N(100)),
			Connections:   ptr(rand.Int64N(100)),
		},
	}

	res := testutil.Put[server.Response[AccountResponse]](t, fixture.AccountURL(acc.NamespaceID, acc.ID), req)
	got := res.Data

	accData, err := acc.Data()
	require.NoError(t, err)
	assert.Equal(t, accData.ID, got.ID)
	assert.Equal(t, accData.OperatorID, got.OperatorID)
	assert.Equal(t, accData.PublicKey, got.PublicKey)

	// Updated fields
	assert.NotEmpty(t, got.JWT)
	assert.NotEqual(t, acc.JWT, got.JWT)
	assert.Equal(t, req.Name, got.Name)
	assert.Equal(t, *req.Limits, got.Limits)
}

func TestServer_GetAccount(t *testing.T) {
	ctx := context.Background()
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	accName := testutil.RandName()
	acc, err := NewAccount(accName, op)
	require.NoError(t, err)
	err = fixture.Store.CreateAccount(ctx, acc)
	require.NoError(t, err)

	res := testutil.Get[server.Response[AccountResponse]](t, fixture.AccountURL(acc.NamespaceID, acc.ID))
	got := res.Data

	accData, err := acc.Data()
	require.NoError(t, err)
	assert.Equal(t, accData, got.AccountData)
	accClaims, err := acc.Claims()
	require.NoError(t, err)

	assert.Equal(t, loadAccountLimits(accData, accClaims), got.Limits)
}

func TestHTTPHandler_ListAccounts(t *testing.T) {
	ctx := context.Background()
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	err = fixture.Store.CreateOperator(ctx, op)
	require.NoError(t, err)

	numOps := 15
	pageSize := 10

	wantResps := make([]AccountResponse, numOps)

	for i := range numOps {
		acc, err := NewAccount(testutil.RandName(), op)
		require.NoError(t, err)
		err = fixture.Store.CreateAccount(ctx, acc)
		require.NoError(t, err)

		wantData, err := acc.Data()
		require.NoError(t, err)
		accClaims, err := acc.Claims()
		require.NoError(t, err)

		wantResps[i] = AccountResponse{
			AccountData: wantData,
			Limits:      loadAccountLimits(wantData, accClaims),
		}
	}

	// Page 1
	url := fmt.Sprintf("%s?%s=%d", fixture.AccountsURL(op.NamespaceID, op.ID), pageSizeQueryParam, pageSize)
	res := testutil.Get[server.ResponseList[AccountResponse]](t, url)
	assert.Equal(t, wantResps[:pageSize], res.Data)
	assert.NotEmpty(t, res.NextPageCursor)

	// Page 2
	url = fmt.Sprintf("%s&%s=%s", url, pageCursorQueryParam, *res.NextPageCursor)
	res = testutil.Get[server.ResponseList[AccountResponse]](t, url)
	assert.Equal(t, wantResps[pageSize:], res.Data)
	assert.Nil(t, res.NextPageCursor)
}

func TestServer_DeleteAccount(t *testing.T) {
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)

	err = fixture.Store.CreateAccount(t.Context(), acc)
	require.NoError(t, err)

	testutil.Delete(t, fixture.AccountURL(acc.NamespaceID, acc.ID))

	_, err = fixture.Store.ReadAccount(t.Context(), acc.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func setupNewTestSysOperator(ctx context.Context, t *testing.T, testStore *Store) (*Operator, *Account) {
	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
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

func TestServer_CreateUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	err = fixture.Store.CreateAccount(ctx, acc)
	require.NoError(t, err)

	req := CreateUserRequest{
		Name: testutil.RandName(),
		Limits: &UserLimits{
			Subscriptions:   ptr(rand.Int64N(100)),
			PayloadSize:     ptr(rand.Int64N(100)),
			JWTDurationSecs: ptr(rand.Int64N(100000)),
		},
	}

	res := testutil.Post[server.Response[UserResponse]](t, fixture.AccountUsersURL(acc.NamespaceID, acc.ID), req)
	got := res.Data

	assert.False(t, got.ID.IsZero())
	assert.Equal(t, op.ID, got.OperatorID)
	assert.Equal(t, acc.ID, got.AccountID)
	assert.Equal(t, req.Name, got.Name)
	assert.NotEmpty(t, got.JWT)
	assert.Equal(t, *req.Limits, got.Limits)
}

func TestServer_UpdateUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	err = fixture.Store.CreateOperator(ctx, op)
	require.NoError(t, err)

	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	err = fixture.Store.CreateAccount(ctx, acc)
	require.NoError(t, err)

	user, err := NewUser(testutil.RandName(), acc)
	require.NoError(t, err)
	err = fixture.Store.CreateUser(ctx, user)
	require.NoError(t, err)

	req := UpdateUserRequest{
		Name: testutil.RandName(),
		Limits: &UserLimits{
			Subscriptions:   ptr(rand.Int64N(100)),
			PayloadSize:     ptr(rand.Int64N(100)),
			JWTDurationSecs: ptr(rand.Int64N(100000)),
		},
	}

	res := testutil.Put[server.Response[UserResponse]](t, fixture.UserURL(user.NamespaceID, user.ID), req)
	got := res.Data

	userData, err := user.Data()
	require.NoError(t, err)
	assert.Equal(t, userData.ID, got.ID)
	assert.Equal(t, userData.OperatorID, got.OperatorID)

	// Updated fields
	assert.NotEmpty(t, got.JWT)
	assert.NotEqual(t, user.JWT(), got.JWT)
	assert.Equal(t, req.Name, got.Name)
	assert.Equal(t, *req.Limits, got.Limits)
}

func TestServer_GetUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	usr, err := NewUser(testutil.RandName(), acc)
	err = fixture.Store.CreateUser(ctx, usr)
	require.NoError(t, err)

	usrData, err := usr.Data()
	require.NoError(t, err)
	usrClaims, err := usr.Claims()
	require.NoError(t, err)

	res := testutil.Get[server.Response[UserResponse]](t, fixture.UserURL(usr.NamespaceID, usr.ID))
	got := res.Data
	assert.Equal(t, usrData, got.UserData)
	assert.Equal(t, usr.JWT(), got.JWT)

	assert.Equal(t, loadUserLimits(usrData, usrClaims), got.Limits)
}

func TestServer_GetUserCreds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	err = fixture.Store.CreateOperator(ctx, op)
	require.NoError(t, err)

	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	err = fixture.Store.CreateAccount(ctx, acc)
	require.NoError(t, err)

	usr, err := NewUser(testutil.RandName(), acc)
	err = fixture.Store.CreateUser(ctx, usr)
	require.NoError(t, err)

	res := testutil.GetText(t, fixture.UserCredsURL(usr.NamespaceID, usr.ID))

	want, err := jwt.FormatUserConfig(usr.JWT(), usr.NKey().Seed)
	require.NoError(t, err)
	assert.Equal(t, string(want), res)
}

func TestServer_GetServerConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, sysAcc := setupNewTestSysOperator(ctx, t, fixture.Store)
	cfgContent := testutil.GetText(t, fixture.OperatorNATSConfigURL(op.NamespaceID, op.ID))
	cfgContent = sanitizeNATSConfig(t, cfgContent)

	// Write to file and process it
	cfgFile, err := os.CreateTemp(t.TempDir(), "")
	require.NoError(t, err)
	err = os.WriteFile(cfgFile.Name(), []byte(cfgContent), 0666)
	require.NoError(t, err)
	opts, err := natsrv.ProcessConfigFile(cfgFile.Name())
	require.NoError(t, err)

	opData, err := op.Data()
	require.NoError(t, err)
	sysAccData, err := sysAcc.Data()
	require.NoError(t, err)

	assert.Equal(t, opData.PublicKey, opts.TrustedOperators[0].Subject)
	assert.False(t, opts.NoSystemAccount)
	assert.Equal(t, sysAccData.PublicKey, opts.SystemAccount)

	opts.Port = testutil.GetFreePort(t)

	// Start server with config and assert auth
	srv, err := natsrv.NewServer(opts)
	require.NoError(t, err)
	srv.Start()
	defer srv.Shutdown()
	srv.ReadyForConnections(time.Second)
	assertNATSAccountAuth(t, srv.ClientURL(), op, sysAcc)
}

func TestHTTPHandler_ListUsers(t *testing.T) {
	ctx := context.Background()
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	err = fixture.Store.CreateOperator(ctx, op)
	require.NoError(t, err)
	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	err = fixture.Store.CreateAccount(ctx, acc)
	require.NoError(t, err)

	numOps := 15
	pageSize := 10

	wantResps := make([]UserResponse, numOps)

	for i := range numOps {
		user, err := NewUser(testutil.RandName(), acc)
		require.NoError(t, err)
		err = fixture.Store.CreateUser(ctx, user)
		require.NoError(t, err)

		wantData, err := user.Data()
		require.NoError(t, err)
		usrClaims, err := user.Claims()
		require.NoError(t, err)
		wantPubKey, err := user.NKey().KeyPair().PublicKey()
		require.NoError(t, err)

		wantResps[i] = UserResponse{
			UserData:  wantData,
			PublicKey: wantPubKey,
			Limits:    loadUserLimits(wantData, usrClaims),
		}
	}

	// Page 1
	url := fmt.Sprintf("%s?%s=%d", fixture.UsersURL(acc.NamespaceID, acc.ID), pageSizeQueryParam, pageSize)
	res := testutil.Get[server.ResponseList[UserResponse]](t, url)
	assert.Equal(t, wantResps[:pageSize], res.Data)
	assert.NotEmpty(t, res.NextPageCursor)

	// Page 2
	url = fmt.Sprintf("%s&%s=%s", url, pageCursorQueryParam, *res.NextPageCursor)
	res = testutil.Get[server.ResponseList[UserResponse]](t, url)
	assert.Equal(t, wantResps[pageSize:], res.Data)
	assert.Nil(t, res.NextPageCursor)
}

func TestServer_DeleteUser(t *testing.T) {
	fixture := SetupTestFixture(t)
	defer fixture.Stop()

	op, err := NewOperator(testutil.RandName(), NewID[NamespaceID]())
	require.NoError(t, err)
	acc, err := NewAccount(testutil.RandName(), op)
	require.NoError(t, err)
	usr, err := NewUser(testutil.RandName(), acc)
	require.NoError(t, err)

	err = fixture.Store.CreateUser(t.Context(), usr)
	require.NoError(t, err)

	testutil.Delete(t, fixture.UserURL(usr.NamespaceID, usr.ID))

	_, err = fixture.Store.ReadUser(t.Context(), usr.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func assertNATSAccountAuth(t *testing.T, natsURL string, operator *Operator, authorizedAcc *Account) {
	t.Helper()
	unauthorizedAcc, err := NewAccount(testutil.RandName(), operator)
	require.NoError(t, err)
	unauthorizedUser, err := NewUser(testutil.RandName(), unauthorizedAcc)
	require.NoError(t, err)

	// Unauthorized user rejected
	nc, err := nats.Connect(natsURL,
		nats.MaxReconnects(0),
		nats.Timeout(200*time.Millisecond),
		nats.UserJWTAndSeed(unauthorizedUser.JWT(), string(unauthorizedUser.NKey().Seed)),
	)
	assert.ErrorContains(t, err, "i/o timeout")

	authorizedUsr, err := NewUser(testutil.RandName(), authorizedAcc)
	require.NoError(t, err)

	// Authorized user accepted
	nc, err = nats.Connect(natsURL, nats.UserJWTAndSeed(authorizedUsr.JWT(), string(authorizedUsr.NKey().Seed)))
	require.NoError(t, err)
	defer nc.Close()

	msgs := make(chan *nats.Msg, 1)

	subj := "test.subject"
	sub, err := nc.ChanSubscribe(subj, msgs)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	data := []byte("Hello World")
	err = nc.Publish(subj, data)
	require.NoError(t, err)

	err = nc.Flush()
	require.NoError(t, err)

	select {
	case msg := <-msgs:
		require.Equal(t, data, msg.Data)
	case <-time.After(200 * time.Millisecond):
		require.Fail(t, "timed out")
	}
}

// Hack: overwrite resolver dir path to avoid unwanted items in the current dir
func sanitizeNATSConfig(t *testing.T, cfgContent string) string {
	return strings.ReplaceAll(cfgContent, defaultResolverDir, t.TempDir())
}
