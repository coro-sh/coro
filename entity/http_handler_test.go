package entity_test // avoid import cycle with sqlite package

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/joshjon/kit/errtag"
	"github.com/joshjon/kit/paginate"
	"github.com/joshjon/kit/ref"
	"github.com/joshjon/kit/server"
	"github.com/nats-io/jwt/v2"
	natsrv "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/testutil"
)

func TestServer_CreateNamespace(t *testing.T) {
	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()

	req := entity.CreateNamespaceRequest{
		Name: testutil.RandName(),
	}

	res := testutil.Post[server.Response[entity.NamespaceResponse]](t, fixture.NamespacesURL(), req)
	got := res.Data
	assert.False(t, got.ID.IsZero())
	assert.Equal(t, req.Name, got.Name)
}

func TestServer_DeleteNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	ns := fixture.AddNamespace(ctx)

	testutil.Delete(t, fixture.NamespaceURL(ns.ID))

	_, err := fixture.Store.ReadNamespaceByName(t.Context(), ns.Name, constants.DefaultNamespaceOwner)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestHTTPHandler_ListNamespaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()

	numNS := 15
	pageSize := 10

	wantResps := make([]entity.NamespaceResponse, numNS)

	for i := range numNS {
		ns := fixture.AddNamespace(ctx)

		res := entity.NamespaceResponse{Namespace: *ns}
		res.Owner = "" // omitted from response
		wantResps[i] = res
	}

	slices.Reverse(wantResps) // expect order by newest to oldest

	// Page 1
	url := fmt.Sprintf("%s?%s=%d", fixture.NamespacesURL(), paginate.PageSizeQueryParam, pageSize)
	res := testutil.Get[server.ResponseList[entity.NamespaceResponse]](t, url)
	assert.Equal(t, wantResps[:pageSize], res.Data)
	assert.NotEmpty(t, res.NextPageCursor)

	// Page 2
	url = fmt.Sprintf("%s&%s=%s", url, paginate.PageCursorQueryParam, *res.NextPageCursor)
	res = testutil.Get[server.ResponseList[entity.NamespaceResponse]](t, url)
	assert.Equal(t, wantResps[pageSize:], res.Data)
	assert.Nil(t, res.NextPageCursor)
}

func TestServer_CreateOperator(t *testing.T) {
	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	ns := fixture.AddNamespace(t.Context())

	req := entity.CreateOperatorRequest{
		Name: testutil.RandName(),
	}

	res := testutil.Post[server.Response[entity.OperatorResponse]](t, fixture.OperatorsURL(ns.ID), req)
	got := res.Data
	assert.False(t, got.ID.IsZero())
	assert.NotEmpty(t, got.JWT)
	assert.Equal(t, req.Name, got.Name)
	assert.NotEmpty(t, got.PublicKey)
}

func TestServer_UpdateOperator(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	op := fixture.AddOperator(ctx)

	req := entity.UpdateOperatorRequest{
		Name: testutil.RandName(),
	}

	res := testutil.Put[server.Response[entity.OperatorResponse]](t, fixture.OperatorURL(op.NamespaceID, op.ID), req)
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
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	op := fixture.AddOperator(ctx)

	res := testutil.Get[server.Response[entity.OperatorResponse]](t, fixture.OperatorURL(op.NamespaceID, op.ID))
	got := res.Data
	opData, err := op.Data()
	require.NoError(t, err)
	assert.Equal(t, opData, got.OperatorData)
}

func TestHTTPHandler_ListOperators(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	ns := fixture.AddNamespace(ctx)

	numOps := 15
	pageSize := 10
	wantResps := make([]entity.OperatorResponse, numOps)

	for i := range numOps {
		op, err := entity.NewOperator(testutil.RandName(), ns.ID)
		require.NoError(t, err)
		err = fixture.Store.CreateOperator(ctx, op)
		require.NoError(t, err)

		wantData, err := op.Data()
		require.NoError(t, err)

		wantResps[i] = entity.OperatorResponse{
			OperatorData: wantData,
			Status: entity.OperatorNATSStatus{
				Connected:   true,
				ConnectTime: ref.Ptr(stubNotifConnectTime),
			},
		}
	}

	slices.Reverse(wantResps) // expect order by newest to oldest

	// Page 1
	url := fmt.Sprintf("%s?%s=%d", fixture.OperatorsURL(ns.ID), paginate.PageSizeQueryParam, pageSize)
	res := testutil.Get[server.ResponseList[entity.OperatorResponse]](t, url)
	assert.Equal(t, wantResps[:pageSize], res.Data)
	assert.NotEmpty(t, res.NextPageCursor)

	// Page 2
	url = fmt.Sprintf("%s&%s=%s", url, paginate.PageCursorQueryParam, *res.NextPageCursor)
	res = testutil.Get[server.ResponseList[entity.OperatorResponse]](t, url)
	assert.Equal(t, wantResps[pageSize:], res.Data)
	assert.Nil(t, res.NextPageCursor)
}

func TestServer_DeleteOperator(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	op := fixture.AddOperator(ctx)

	testutil.Delete(t, fixture.OperatorURL(op.NamespaceID, op.ID))

	_, err := fixture.Store.ReadOperator(t.Context(), op.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func TestServer_CreateAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	op := fixture.AddOperator(ctx)

	req := entity.CreateAccountRequest{
		Name: testutil.RandName(),
		Limits: &entity.AccountLimits{
			Subscriptions:       ref.Ptr(rand.Int64N(100)),
			PayloadSize:         ref.Ptr(rand.Int64N(100)),
			Imports:             ref.Ptr(rand.Int64N(100)),
			Exports:             ref.Ptr(rand.Int64N(100)),
			Connections:         ref.Ptr(rand.Int64N(100)),
			UserJWTDurationSecs: ref.Ptr(rand.Int64N(100000)),
		},
	}

	res := testutil.Post[server.Response[entity.AccountResponse]](t, fixture.OperatorAccountsURL(op.NamespaceID, op.ID), req)
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

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	acc := fixture.AddAccount(ctx)

	req := entity.UpdateAccountRequest{
		Name: testutil.RandName(),
		Limits: &entity.AccountLimits{
			Subscriptions: ref.Ptr(rand.Int64N(100)),
			PayloadSize:   ref.Ptr(rand.Int64N(100)),
			Imports:       ref.Ptr(rand.Int64N(100)),
			Exports:       ref.Ptr(rand.Int64N(100)),
			Connections:   ref.Ptr(rand.Int64N(100)),
		},
	}

	res := testutil.Put[server.Response[entity.AccountResponse]](t, fixture.AccountURL(acc.NamespaceID, acc.ID), req)
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
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	acc := fixture.AddAccount(ctx)

	res := testutil.Get[server.Response[entity.AccountResponse]](t, fixture.AccountURL(acc.NamespaceID, acc.ID))
	got := res.Data

	accData, err := acc.Data()
	require.NoError(t, err)
	assert.Equal(t, accData, got.AccountData)
	accClaims, err := acc.Claims()
	require.NoError(t, err)

	assert.Equal(t, entity.LoadAccountLimits(accData, accClaims), got.Limits)
}

func TestHTTPHandler_ListAccounts(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	op := fixture.AddOperator(ctx)

	numOps := 15
	pageSize := 10

	wantResps := make([]entity.AccountResponse, numOps)

	for i := range numOps {
		acc, err := entity.NewAccount(testutil.RandName(), op)
		require.NoError(t, err)
		err = fixture.Store.CreateAccount(ctx, acc)
		require.NoError(t, err)

		wantData, err := acc.Data()
		require.NoError(t, err)
		accClaims, err := acc.Claims()
		require.NoError(t, err)

		wantResps[i] = entity.AccountResponse{
			AccountData: wantData,
			Limits:      entity.LoadAccountLimits(wantData, accClaims),
		}
	}

	slices.Reverse(wantResps) // expect order by newest to oldest

	// Page 1
	url := fmt.Sprintf("%s?%s=%d", fixture.AccountsURL(op.NamespaceID, op.ID), paginate.PageSizeQueryParam, pageSize)
	res := testutil.Get[server.ResponseList[entity.AccountResponse]](t, url)
	assert.Equal(t, wantResps[:pageSize], res.Data)
	assert.NotEmpty(t, res.NextPageCursor)

	// Page 2
	url = fmt.Sprintf("%s&%s=%s", url, paginate.PageCursorQueryParam, *res.NextPageCursor)
	res = testutil.Get[server.ResponseList[entity.AccountResponse]](t, url)
	assert.Equal(t, wantResps[pageSize:], res.Data)
	assert.Nil(t, res.NextPageCursor)
}

func TestServer_DeleteAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	acc := fixture.AddAccount(ctx)

	testutil.Delete(t, fixture.AccountURL(acc.NamespaceID, acc.ID))

	_, err := fixture.Store.ReadAccount(t.Context(), acc.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func setupNewTestSysOperator(ctx context.Context, t *testing.T, store *entity.Store) (*entity.Operator, *entity.Account) {
	ns := entity.NewNamespace(testutil.RandName(), constants.DefaultNamespaceOwner)
	require.NoError(t, store.CreateNamespace(ctx, ns))
	op, err := entity.NewOperator(testutil.RandName(), ns.ID)
	require.NoError(t, err)
	sysAcc, sysUser, err := op.SetNewSystemAccountAndUser()
	require.NoError(t, err)
	require.NoError(t, store.CreateOperator(ctx, op))
	require.NoError(t, store.CreateAccount(ctx, sysAcc))
	require.NoError(t, store.CreateUser(ctx, sysUser))
	return op, sysAcc
}

func TestServer_CreateUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	acc := fixture.AddAccount(ctx)

	req := entity.CreateUserRequest{
		Name: testutil.RandName(),
		Limits: &entity.UserLimits{
			Subscriptions:   ref.Ptr(rand.Int64N(100)),
			PayloadSize:     ref.Ptr(rand.Int64N(100)),
			JWTDurationSecs: ref.Ptr(rand.Int64N(100000)),
		},
	}

	res := testutil.Post[server.Response[entity.UserResponse]](t, fixture.AccountUsersURL(acc.NamespaceID, acc.ID), req)
	got := res.Data

	assert.False(t, got.ID.IsZero())
	assert.Equal(t, acc.OperatorID, got.OperatorID)
	assert.Equal(t, acc.ID, got.AccountID)
	assert.Equal(t, req.Name, got.Name)
	assert.NotEmpty(t, got.JWT)
	assert.Equal(t, *req.Limits, got.Limits)
}

func TestServer_UpdateUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	usr := fixture.AddUser(ctx)

	req := entity.UpdateUserRequest{
		Name: testutil.RandName(),
		Limits: &entity.UserLimits{
			Subscriptions:   ref.Ptr(rand.Int64N(100)),
			PayloadSize:     ref.Ptr(rand.Int64N(100)),
			JWTDurationSecs: ref.Ptr(rand.Int64N(100000)),
		},
	}

	res := testutil.Put[server.Response[entity.UserResponse]](t, fixture.UserURL(usr.NamespaceID, usr.ID), req)
	got := res.Data

	userData, err := usr.Data()
	require.NoError(t, err)
	assert.Equal(t, userData.ID, got.ID)
	assert.Equal(t, userData.OperatorID, got.OperatorID)

	// Updated fields
	assert.NotEmpty(t, got.JWT)
	assert.NotEqual(t, usr.JWT(), got.JWT)
	assert.Equal(t, req.Name, got.Name)
	assert.Equal(t, *req.Limits, got.Limits)
}

func TestServer_GetUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	usr := fixture.AddUser(ctx)

	usrData, err := usr.Data()
	require.NoError(t, err)
	usrClaims, err := usr.Claims()
	require.NoError(t, err)

	res := testutil.Get[server.Response[entity.UserResponse]](t, fixture.UserURL(usr.NamespaceID, usr.ID))
	got := res.Data
	assert.Equal(t, usrData, got.UserData)
	assert.Equal(t, usr.JWT(), got.JWT)

	assert.Equal(t, entity.LoadUserLimits(usrData, usrClaims), got.Limits)
}

func TestServer_GetUserCreds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	usr := fixture.AddUser(ctx)

	res := testutil.GetText(t, fixture.UserCredsURL(usr.NamespaceID, usr.ID))

	want, err := jwt.FormatUserConfig(usr.JWT(), usr.NKey().Seed)
	require.NoError(t, err)
	assert.Equal(t, string(want), res)
}

func TestServer_GetServerConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()

	op, sysAcc := setupNewTestSysOperator(ctx, t, fixture.Store)
	cfgContent := testutil.GetText(t, fixture.OperatorNATSConfigURL(op.NamespaceID, op.ID))
	cfgContent = sanitizeNATSConfig(t, cfgContent)

	// Write to file and process it
	cfgFile, err := os.CreateTemp(t.TempDir(), "")
	require.NoError(t, err)
	defer cfgFile.Close()
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
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	acc := fixture.AddAccount(ctx)

	numOps := 15
	pageSize := 10

	wantResps := make([]entity.UserResponse, numOps)

	for i := range numOps {
		user, err := entity.NewUser(testutil.RandName(), acc)
		require.NoError(t, err)
		err = fixture.Store.CreateUser(ctx, user)
		require.NoError(t, err)

		wantData, err := user.Data()
		require.NoError(t, err)
		usrClaims, err := user.Claims()
		require.NoError(t, err)
		wantPubKey, err := user.NKey().KeyPair().PublicKey()
		require.NoError(t, err)

		wantResps[i] = entity.UserResponse{
			UserData:  wantData,
			PublicKey: wantPubKey,
			Limits:    entity.LoadUserLimits(wantData, usrClaims),
		}
	}

	slices.Reverse(wantResps) // expect order by newest to oldest

	// Page 1
	url := fmt.Sprintf("%s?%s=%d", fixture.UsersURL(acc.NamespaceID, acc.ID), paginate.PageSizeQueryParam, pageSize)
	res := testutil.Get[server.ResponseList[entity.UserResponse]](t, url)
	assert.Equal(t, wantResps[:pageSize], res.Data)
	assert.NotEmpty(t, res.NextPageCursor)

	// Page 2
	url = fmt.Sprintf("%s&%s=%s", url, paginate.PageCursorQueryParam, *res.NextPageCursor)
	res = testutil.Get[server.ResponseList[entity.UserResponse]](t, url)
	assert.Equal(t, wantResps[pageSize:], res.Data)
	assert.Nil(t, res.NextPageCursor)
}

func TestServer_DeleteUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	fixture := NewHTTPHandlerTestFixture(t)
	defer fixture.Stop()
	usr := fixture.AddUser(ctx)

	testutil.Delete(t, fixture.UserURL(usr.NamespaceID, usr.ID))

	_, err := fixture.Store.ReadUser(t.Context(), usr.ID)
	assert.True(t, errtag.HasTag[errtag.NotFound](err))
}

func assertNATSAccountAuth(t *testing.T, natsURL string, operator *entity.Operator, authorizedAcc *entity.Account) {
	t.Helper()
	unauthorizedAcc, err := entity.NewAccount(testutil.RandName(), operator)
	require.NoError(t, err)
	unauthorizedUser, err := entity.NewUser(testutil.RandName(), unauthorizedAcc)
	require.NoError(t, err)

	// Unauthorized user rejected
	nc, err := nats.Connect(natsURL,
		nats.MaxReconnects(0),
		nats.Timeout(200*time.Millisecond),
		nats.UserJWTAndSeed(unauthorizedUser.JWT(), string(unauthorizedUser.NKey().Seed)),
	)
	assert.ErrorContains(t, err, "i/o timeout")

	authorizedUsr, err := entity.NewUser(testutil.RandName(), authorizedAcc)
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
	return strings.ReplaceAll(cfgContent, entity.DefaultResolverDir, t.TempDir())
}
