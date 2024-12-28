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
	"github.com/labstack/echo/v4"

	"github.com/coro-sh/coro/broker"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/internal/constants"
	"github.com/coro-sh/coro/log"
	"github.com/coro-sh/coro/notif"
	"github.com/coro-sh/coro/postgres"
	"github.com/coro-sh/coro/proxy"
	"github.com/coro-sh/coro/server"
	"github.com/coro-sh/coro/tkn"
)

func RunAll(ctx context.Context, logger log.Logger, cfg AllConfig, withUI bool) error {
	pgDialOps := getPostgresDialOpts(cfg.Postgres)
	pg, err := postgres.Dial(ctx, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.HostPort, postgres.AppDBName, pgDialOps...)
	if err != nil {
		return err
	}
	defer pg.Close()

	txer := postgres.NewTxer(pg)
	store, err := newEntityStore(txer, postgres.NewEntityRepository(pg), cfg.EncryptionSecretKey)
	if err != nil {
		return err
	}

	intNS, err := initNamespace(ctx, store, logger, constants.InternalNamespaceName)
	if err != nil {
		return err
	}
	if _, err = initNamespace(ctx, store, logger, constants.DefaultNamespaceName); err != nil {
		return err
	}

	bOp, bSysAcc, bSysUsr, err := initBrokerNATSEntities(ctx, txer, store, logger, intNS.ID)
	if err != nil {
		return err
	}

	brokerNats, err := startEmbeddedNATS(logger, bOp, bSysAcc, nil, cfg.TLS)
	if err != nil {
		return fmt.Errorf("create broker embedded nats server: %w", err)
	}
	defer brokerNats.Shutdown()

	opTknRW := postgres.NewOperatorTokenReadWriter(pg)
	opTknIssuer := tkn.NewOperatorIssuer(opTknRW, tkn.OperatorTokenTypeProxy)

	// Server
	srvOpts := []server.Option{
		server.WithLogger(logger),
		server.WithMiddleware(
			entity.NamespaceContextMiddleware(),
			entity.InternalNamespaceMiddleware(intNS.ID),
		),
	}
	if len(cfg.CorsOrigins) > 0 {
		srvOpts = append(srvOpts, server.WithCORS(cfg.CorsOrigins...))
	}
	if cfg.TLS != nil {
		srvOpts = append(srvOpts, server.WithTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CACertFile))
	}
	srv, err := server.NewServer(cfg.Port, srvOpts...)
	if err != nil {
		return err
	}

	brokerHandler, err := broker.NewWebSocketHandler(bSysUsr, brokerNats, opTknIssuer, store, broker.WithLogger(logger))
	if err != nil {
		return err
	}

	brokerPub, err := broker.DialPublisher("", bSysUsr, broker.WithPublisherEmbeddedNATS(brokerNats))
	if err != nil {
		return fmt.Errorf("dial broker publisher: %w", err)
	}

	notif, err := notif.NewNotifier(store, brokerPub)
	if err != nil {
		return err
	}

	srv.Register(entity.NewHTTPHandler(txer, store, entity.WithNotifier(notif)))
	srv.Register(brokerHandler)
	srv.Register(proxy.NewHTTPHandler(opTknIssuer, store, notif))

	if withUI {
		var uiHandler, err = uiserver.AssetsHandler()
		if err != nil {
			return err
		}
		srv.Add(echo.GET, "/*", uiHandler)
	}

	return serve(ctx, srv, logger)
}

func RunUI(ctx context.Context, logger log.Logger, cfg UIConfig) error {
	srvOpts := []server.Option{server.WithLogger(logger)}
	if len(cfg.CorsOrigins) > 0 {
		srvOpts = append(srvOpts, server.WithCORS(cfg.CorsOrigins...))
	}
	if cfg.TLS != nil {
		srvOpts = append(srvOpts, server.WithTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CACertFile))
	}
	srv, err := server.NewServer(cfg.Port, srvOpts...)
	if err != nil {
		return err
	}

	uiHandler, err := uiserver.AssetsHandler()
	if err != nil {
		return err
	}
	srv.Add(echo.GET, "/*", uiHandler)
	httpClient, err := createHTTPClient(cfg.TLS)
	if err != nil {
		return err
	}
	srv.Any("/api/*", apiProxyHandler(httpClient, cfg.APIAddress))

	return serve(ctx, srv, logger)
}

func RunController(ctx context.Context, logger log.Logger, cfg ControllerConfig) error {
	pgDialOps := getPostgresDialOpts(cfg.Postgres)
	pg, err := postgres.Dial(ctx, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.HostPort, postgres.AppDBName, pgDialOps...)
	if err != nil {
		return err
	}
	defer pg.Close()

	txer := postgres.NewTxer(pg)
	store, err := newEntityStore(txer, postgres.NewEntityRepository(pg), cfg.EncryptionSecretKey)
	if err != nil {
		return err
	}

	intNS, err := initNamespace(ctx, store, logger, constants.InternalNamespaceName)
	if err != nil {
		return err
	}
	if _, err = initNamespace(ctx, store, logger, constants.DefaultNamespaceName); err != nil {
		return err
	}

	srvOpts := []server.Option{
		server.WithLogger(logger),
		server.WithMiddleware(
			entity.NamespaceContextMiddleware(),
			entity.InternalNamespaceMiddleware(intNS.ID),
		),
	}
	if len(cfg.CorsOrigins) > 0 {
		srvOpts = append(srvOpts, server.WithCORS(cfg.CorsOrigins...))
	}
	if cfg.TLS != nil {
		srvOpts = append(srvOpts, server.WithTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CACertFile))
	}
	srv, err := server.NewServer(cfg.Port, srvOpts...)
	if err != nil {
		return err
	}

	var entityHandlerOpts []entity.HTTPHandlerOption
	if cfg.Broker != nil {
		_, _, bSysUsr, err := initBrokerNATSEntities(ctx, txer, store, logger, intNS.ID)
		if err != nil {
			return err
		}
		var pubOpts []broker.PublisherOption
		if cfg.TLS != nil {
			pubOpts = append(pubOpts, broker.WithPublisherTLS(broker.TLSConfig{
				CertFile:           cfg.TLS.CertFile,
				KeyFile:            cfg.TLS.KeyFile,
				CACertFile:         cfg.TLS.CACertFile,
				InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
			}))
		}
		brokerPub, err := broker.DialPublisher(strings.Join(cfg.Broker.NatsURLs, ","), bSysUsr, pubOpts...)
		if err != nil {
			return fmt.Errorf("dial broker publisher: %w", err)
		}

		notif, err := notif.NewNotifier(store, brokerPub)
		if err != nil {
			return err
		}
		entityHandlerOpts = append(entityHandlerOpts, entity.WithNotifier(notif))
		opTknRW := postgres.NewOperatorTokenReadWriter(pg)
		opTknIssuer := tkn.NewOperatorIssuer(opTknRW, tkn.OperatorTokenTypeProxy)
		srv.Register(proxy.NewHTTPHandler(opTknIssuer, store, notif))
	}

	srv.Register(entity.NewHTTPHandler(txer, store, entityHandlerOpts...))

	return serve(ctx, srv, logger)
}

func RunBroker(ctx context.Context, logger log.Logger, cfg BrokerConfig) error {
	pgDialOps := getPostgresDialOpts(cfg.Postgres)
	pg, err := postgres.Dial(ctx, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.HostPort, postgres.AppDBName, pgDialOps...)
	if err != nil {
		return err
	}
	defer pg.Close()

	txer := postgres.NewTxer(pg)
	store, err := newEntityStore(txer, postgres.NewEntityRepository(pg), cfg.EncryptionSecretKey)
	if err != nil {
		return fmt.Errorf("create entity store: %w", err)
	}

	intNS, err := initNamespace(ctx, store, logger, constants.InternalNamespaceName)
	if err != nil {
		return err
	}
	bOp, bSysAcc, bSysUsr, err := initBrokerNATSEntities(ctx, txer, store, logger, intNS.ID)
	if err != nil {
		return err
	}

	opTknIssuer := tkn.NewOperatorIssuer(postgres.NewOperatorTokenReadWriter(pg), tkn.OperatorTokenTypeProxy)

	brokerNats, err := startEmbeddedNATS(logger, bOp, bSysAcc, &cfg.EmbeddedNats, cfg.TLS)
	if err != nil {
		return fmt.Errorf("create broker embedded nats server: %w", err)
	}
	defer brokerNats.Shutdown()

	handler, err := broker.NewWebSocketHandler(
		bSysUsr,
		brokerNats,
		opTknIssuer,
		store,
		broker.WithLogger(logger),
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

	srv, err := server.NewServer(cfg.Port, srvOpts...)
	if err != nil {
		return err
	}

	srv.Register(handler)

	return serve(ctx, srv, logger)
}

func createHTTPClient(tlsCfg *TLSConfig) (*http.Client, error) {
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

func getPostgresDialOpts(cfg PostgresConfig) []postgres.DialOption {
	var pgOpts []postgres.DialOption
	if cfg.TLS != nil {
		pgOpts = append(pgOpts, postgres.WithTLS(postgres.TLSConfig{
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
