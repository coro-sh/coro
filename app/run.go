package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	uiserver "github.com/coro-sh/coro-ui-server"
	"github.com/joshjon/kit/pgdb"
	"github.com/joshjon/kit/sqlitedb"
	"github.com/labstack/echo/v4"

	"github.com/joshjon/kit/log"
	"github.com/joshjon/kit/server"

	"github.com/coro-sh/coro/command"
	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/entityapi"
	"github.com/coro-sh/coro/postgres"
	"github.com/coro-sh/coro/proxyapi"
	"github.com/coro-sh/coro/sqlite"
	"github.com/coro-sh/coro/sqlite/migrations"
	"github.com/coro-sh/coro/streamapi"
	"github.com/coro-sh/coro/tkn"
)

func RunAll(ctx context.Context, logger log.Logger, cfg AllConfig, withUI bool, opts ...server.Option) error {
	pgDialOps := getPostgresDialOpts(cfg.Postgres)
	pg, err := pgdb.Dial(ctx, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.HostPort, cfg.Postgres.Database, pgDialOps...)
	if err != nil {
		return err
	}
	defer pg.Close()
	return runAll(ctx, logger, cfg, withUI, postgres.NewEntityRepository(pg), postgres.NewOperatorTokenReadWriter(pg), opts...)
}

func RunUI(ctx context.Context, logger log.Logger, cfg UIConfig, opts ...server.Option) error {
	srvOpts := []server.Option{server.WithLogger(logger)}
	if cfg.TLS != nil {
		srvOpts = append(srvOpts, server.WithTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CACertFile))
	}
	srvOpts = append(srvOpts, opts...)
	srv, err := server.NewServer(cfg.Port, srvOpts...)
	if err != nil {
		return err
	}

	uiHandler, err := uiserver.DistHandler(uiserver.BuildCloud)
	if err != nil {
		return err
	}
	srv.Add(echo.GET, "/*", uiHandler)
	httpClient, err := NewHTTPClient(cfg.TLS)
	if err != nil {
		return err
	}
	srv.Any("/api/*", apiProxyHandler(httpClient, cfg.APIAddress))

	return Serve(ctx, srv, logger)
}

func RunController(ctx context.Context, logger log.Logger, cfg ControllerConfig, opts ...server.Option) error {
	pgDialOps := getPostgresDialOpts(cfg.Postgres)
	pg, err := pgdb.Dial(ctx, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.HostPort, cfg.Postgres.Database, pgDialOps...)
	if err != nil {
		return err
	}
	defer pg.Close()

	store, err := NewEntityStore(postgres.NewEntityRepository(pg), cfg.EncryptionSecretKey)
	if err != nil {
		return err
	}

	intNS, err := InitNamespace(ctx, store, logger, constants.InternalNamespaceName, constants.InternalNamespaceOwner)
	if err != nil {
		return err
	}
	if _, err = InitNamespace(ctx, store, logger, constants.DefaultNamespaceName, constants.DefaultNamespaceOwner); err != nil {
		return err
	}

	srvOpts := []server.Option{
		server.WithLogger(logger),
		server.WithMiddleware(
			entityapi.NamespaceContextMiddleware(),
			entityapi.InternalNamespaceMiddleware(intNS.ID),
		),
	}
	if len(cfg.CorsOrigins) > 0 {
		srvOpts = append(srvOpts, server.WithCORS(cfg.CorsOrigins...))
	}
	if cfg.TLS != nil {
		srvOpts = append(srvOpts, server.WithTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CACertFile))
	}
	srvOpts = append(srvOpts, opts...)
	srv, err := server.NewServer(cfg.Port, srvOpts...)
	if err != nil {
		return err
	}

	var entityHandlerOpts []entityapi.HTTPHandlerOption[*entity.Store]
	if cfg.Broker != nil {
		_, _, bSysUsr, err := InitBrokerNATSEntities(ctx, store, logger, intNS.ID)
		if err != nil {
			return err
		}
		var cmdOpts []command.CommanderOption
		if cfg.TLS != nil {
			cmdOpts = append(cmdOpts, command.WithCommanderTLS(command.TLSConfig{
				CertFile:           cfg.TLS.CertFile,
				KeyFile:            cfg.TLS.KeyFile,
				CACertFile:         cfg.TLS.CACertFile,
				InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
			}))
		}
		commander, err := command.NewCommander(strings.Join(cfg.Broker.NatsURLs, ","), bSysUsr, cmdOpts...)
		if err != nil {
			return fmt.Errorf("dial broker publisher: %w", err)
		}

		entityHandlerOpts = append(entityHandlerOpts, entityapi.WithCommander[*entity.Store](commander))
		opTknRW := postgres.NewOperatorTokenReadWriter(pg)
		opTknIssuer := tkn.NewOperatorIssuer(opTknRW, tkn.OperatorTokenTypeProxy)
		srv.Register("/api/v1", proxyapi.NewProxyHTTPHandler(opTknIssuer, store, commander))
		srv.Register("/api/v1", streamapi.NewStreamHTTPHandler(store, commander))
		srv.Register("/api/v1", streamapi.NewStreamWebSocketHandler(store, commander, streamapi.WithStreamWebSocketHandlerCORS(cfg.CorsOrigins...)))
	}

	srv.Register("/api/v1", entityapi.NewHTTPHandler(store, entityHandlerOpts...))

	return Serve(ctx, srv, logger)
}

func RunBroker(ctx context.Context, logger log.Logger, cfg BrokerConfig, opts ...server.Option) error {
	pgDialOps := getPostgresDialOpts(cfg.Postgres)
	pg, err := pgdb.Dial(ctx, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.HostPort, cfg.Postgres.Database, pgDialOps...)
	if err != nil {
		return err
	}
	defer pg.Close()

	store, err := NewEntityStore(postgres.NewEntityRepository(pg), cfg.EncryptionSecretKey)
	if err != nil {
		return fmt.Errorf("create entity store: %w", err)
	}

	intNS, err := InitNamespace(ctx, store, logger, constants.InternalNamespaceName, constants.InternalNamespaceOwner)
	if err != nil {
		return err
	}
	bOp, bSysAcc, bSysUsr, err := InitBrokerNATSEntities(ctx, store, logger, intNS.ID)
	if err != nil {
		return err
	}

	opTknIssuer := tkn.NewOperatorIssuer(postgres.NewOperatorTokenReadWriter(pg), tkn.OperatorTokenTypeProxy)

	brokerNats, err := StartEmbeddedNATS(logger, bOp, bSysAcc, &cfg.EmbeddedNats, cfg.TLS)
	if err != nil {
		return fmt.Errorf("create broker embedded nats server: %w", err)
	}
	defer brokerNats.Shutdown()

	handler, err := command.NewBrokerWebSocketHandler(
		bSysUsr,
		brokerNats,
		opTknIssuer,
		store,
		command.WithBrokerWebsocketLogger(logger),
	)
	if err != nil {
		return err
	}

	srvOpts := []server.Option{server.WithLogger(logger)}
	if cfg.TLS != nil {
		srvOpts = append(srvOpts, server.WithTLS(
			cfg.TLS.CertFile,
			cfg.TLS.KeyFile,
			cfg.TLS.CACertFile,
		))
	}
	srvOpts = append(srvOpts, opts...)
	srv, err := server.NewServer(cfg.Port, srvOpts...)
	if err != nil {
		return err
	}

	srv.Register("/api/v1", handler)

	return Serve(ctx, srv, logger)
}

func RunDevServer(ctx context.Context, logger log.Logger, serverPort int, withUI bool, corsOrigins ...string) error {
	db, err := sqlitedb.Open(ctx, sqlitedb.WithInMemory())
	if err != nil {
		return err
	}
	defer db.Close()

	if err = sqlitedb.Migrate(db, migrations.FS); err != nil {
		return err
	}

	appCfg := AllConfig{
		BaseConfig: BaseConfig{
			Port: serverPort,
			Logger: LoggerConfig{
				Level:      "debug",
				Structured: false,
			},
		},
		CorsOrigins: corsOrigins,
	}

	return runAll(ctx, logger, appCfg, withUI, sqlite.NewEntityRepository(db), sqlite.NewOperatorTokenReadWriter(db))
}

func runAll(
	ctx context.Context,
	logger log.Logger,
	cfg AllConfig,
	withUI bool,
	repo entity.Repository,
	opTknRW tkn.OperatorTokenReadWriter,
	opts ...server.Option,
) error {
	store, err := NewEntityStore(repo, cfg.EncryptionSecretKey)
	if err != nil {
		return err
	}

	intNS, err := InitNamespace(ctx, store, logger, constants.InternalNamespaceName, constants.InternalNamespaceOwner)
	if err != nil {
		return err
	}
	if _, err = InitNamespace(ctx, store, logger, constants.DefaultNamespaceName, constants.DefaultNamespaceOwner); err != nil {
		return err
	}

	bOp, bSysAcc, bSysUsr, err := InitBrokerNATSEntities(ctx, store, logger, intNS.ID)
	if err != nil {
		return err
	}

	brokerNats, err := StartEmbeddedNATS(logger, bOp, bSysAcc, nil, cfg.TLS)
	if err != nil {
		return fmt.Errorf("create broker embedded nats server: %w", err)
	}
	defer brokerNats.Shutdown()

	opTknIssuer := tkn.NewOperatorIssuer(opTknRW, tkn.OperatorTokenTypeProxy)

	// Server
	srvOpts := []server.Option{
		server.WithLogger(logger),
		server.WithCORS(cfg.CorsOrigins...),
		server.WithMiddleware(
			entityapi.NamespaceContextMiddleware(),
			entityapi.InternalNamespaceMiddleware(intNS.ID),
		),
	}
	if cfg.TLS != nil {
		srvOpts = append(srvOpts, server.WithTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CACertFile))
	}
	srvOpts = append(srvOpts, opts...)
	srv, err := server.NewServer(cfg.Port, srvOpts...)
	if err != nil {
		return err
	}

	brokerHandler, err := command.NewBrokerWebSocketHandler(bSysUsr, brokerNats, opTknIssuer, store, command.WithBrokerWebsocketLogger(logger))
	if err != nil {
		return err
	}

	commander, err := command.NewCommander("", bSysUsr, command.WithCommanderEmbeddedNATS(brokerNats))
	if err != nil {
		return fmt.Errorf("dial broker publisher: %w", err)
	}

	srv.Register("/api/v1", entityapi.NewHTTPHandler(store, entityapi.WithCommander[*entity.Store](commander)))
	srv.Register("/api/v1", brokerHandler)
	srv.Register("/api/v1", proxyapi.NewProxyHTTPHandler(opTknIssuer, store, commander))
	srv.Register("/api/v1", streamapi.NewStreamHTTPHandler(store, commander))
	srv.Register("/api/v1", streamapi.NewStreamWebSocketHandler(store, commander, streamapi.WithStreamWebSocketHandlerCORS(cfg.CorsOrigins...)))

	if withUI {
		var uiHandler, err = uiserver.DistHandler(uiserver.BuildLocal)
		if err != nil {
			return err
		}
		srv.Add(echo.GET, "/*", uiHandler)
	}

	return Serve(ctx, srv, logger)
}

func NewHTTPClient(tlsCfg *TLSConfig) (*http.Client, error) {
	client := http.DefaultClient
	client.Timeout = 60 * time.Second

	if tlsCfg == nil {
		return client, nil
	}

	cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
	if err != nil {
		return nil, err
	}

	caCert, err := os.ReadFile(tlsCfg.CACertFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to append ca cert")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client.Transport = transport
	return client, nil
}

func getPostgresDialOpts(cfg PostgresConfig) []pgdb.DialOption {
	var pgOpts []pgdb.DialOption
	if cfg.TLS != nil {
		pgOpts = append(pgOpts, pgdb.WithTLS(pgdb.TLSConfig{
			CertFile:           cfg.TLS.CertFile,
			KeyFile:            cfg.TLS.KeyFile,
			CACertFile:         cfg.TLS.CACertFile,
			InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		}))
	}
	return pgOpts
}

func apiProxyHandler(client *http.Client, apiURL string) echo.HandlerFunc {
	return func(c echo.Context) error {
		targetURL, err := url.Parse(apiURL)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Bad target URL")
		}
		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		proxy.Transport = client.Transport
		proxy.ServeHTTP(c.Response().Writer, c.Request())
		return nil
	}
}
