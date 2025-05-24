package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/cohesivestack/valgo"
	"github.com/urfave/cli/v2"

	"github.com/coro-sh/coro/app"
	"github.com/coro-sh/coro/client"
	"github.com/coro-sh/coro/internal/valgoutil"
	"github.com/coro-sh/coro/log"
	"github.com/coro-sh/coro/postgres/migrations"
)

const (
	clientMaxReconnectAttempts = 10
	clientReconnectDelay       = 500 * time.Millisecond
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	logger := log.NewLogger(log.WithDevelopment())

	if err := run(ctx, os.Args, logger); err != nil {
		logger.Error("dev server failed", "error", err)
		os.Exit(1)
	}
}

type config struct {
	port             int
	postgresHostPort string
	postgresUser     string
	postgresPassword string
	enableUI         bool
	corsOrigins      []string
}

func (c config) validate() *valgo.Validation {
	v := valgo.New()

	for i, origin := range c.corsOrigins {
		v.InRow("corsOrigins", i, valgo.Is(valgoutil.URLValidator(origin, "origin")))
	}

	return v.Is(
		valgo.Int(c.port, "port").Not().Zero(),
		valgoutil.HostPortValidator(c.postgresHostPort, "postgres-hostport"),
		valgo.String(c.postgresUser, "postgres-user").Not().Blank(),
		valgo.String(c.postgresPassword, "postgres-password").Not().Blank(),
	)
}

func loadConfig(c *cli.Context) config {
	cfg := config{
		port:             c.Int("port"),
		postgresHostPort: c.String("postgres-hostport"),
		postgresUser:     c.String("postgres-user"),
		postgresPassword: c.String("postgres-password"),
		enableUI:         c.Bool("server-ui"),
		corsOrigins:      c.StringSlice("cors-origin"),
	}
	exitOnInvalidFlags(c, cfg.validate())
	return cfg
}

func run(ctx context.Context, args []string, logger log.Logger) error {
	app := cli.NewApp()
	app.Name = "coro-dev-server"
	app.Usage = "All-in-one backend server to use when developing the UI."

	app.Flags = []cli.Flag{
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   6400,
			Usage:   "port to run the server on",
			EnvVars: []string{"PORT"},
		},
		&cli.StringFlag{
			Name:    "postgres-hostport",
			Aliases: []string{"ph"},
			Value:   "127.0.0.1:5432",
			Usage:   "hostport of postgres",
			EnvVars: []string{"POSTGRES_HOST_PORT"},
		},
		&cli.StringFlag{
			Name:    "postgres-user",
			Aliases: []string{"pu"},
			Value:   "postgres",
			Usage:   "username for auth when connecting to postgres",
			EnvVars: []string{"POSTGRES_USER"},
		},
		&cli.StringFlag{
			Name:    "postgres-password",
			Aliases: []string{"pw"},
			Value:   "postgres",
			Usage:   "password for auth when connecting to postgres",
			EnvVars: []string{"POSTGRES_PASSWORD"},
		},
		&cli.BoolFlag{
			Name:    "server-ui",
			Aliases: []string{"ui"},
			Value:   true,
			Usage:   "enable coro server ui",
			EnvVars: []string{"SERVER_UI"},
		},
		&cli.StringSliceFlag{
			Name:    "cors-origin",
			Aliases: []string{"co"},
			Value:   nil,
			Usage:   "add cors origin",
			EnvVars: []string{"CORS_ORIGINS"},
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "run",
			Usage: "[default] runs the development server",
			Action: func(c *cli.Context) error {
				cfg := loadConfig(c)
				return cmdRun(ctx, logger, cfg)
			},
		},
	}

	app.DefaultCommand = "run"

	return app.RunContext(ctx, args)
}

func cmdRun(ctx context.Context, logger log.Logger, cfg config) error {
	errCh := make(chan error)

	pg, err := recreateDB(ctx, cfg, logger)
	if err != nil {
		return err
	}

	logger.Info("migrating database")
	if err = migrations.MigrateDatabase(pg); err != nil {
		return err
	}

	appCfg := app.AllConfig{
		BaseConfig: app.BaseConfig{
			Port: cfg.port,
			Logger: app.LoggerConfig{
				Level:      "debug",
				Structured: false,
			},
			Postgres: app.PostgresConfig{
				HostPort: cfg.postgresHostPort,
				User:     cfg.postgresUser,
				Password: cfg.postgresPassword,
			},
		},
		CorsOrigins: cfg.corsOrigins,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		if err = app.RunAll(ctx, logger, appCfg, cfg.enableUI); err != nil {
			errCh <- err
		}
	}()

	addr := fmt.Sprintf("127.0.0.1:%d/api/v1", cfg.port)
	httpURL := fmt.Sprintf("http://%s", addr)
	wsURL := fmt.Sprintf("ws://%s", addr)
	logger.Info("waiting for client connection to be healthy")
	herrCh := waitForClientConnectionHealthy(ctx, cfg.port, clientMaxReconnectAttempts, clientReconnectDelay)

	select {
	case err = <-errCh:
		return err
	case err = <-herrCh:
		if err != nil {
			return err
		}
		logger.Info("client connection healthy")
	}

	c, err := client.NewClient(httpURL)
	if err != nil {
		return err
	}

	namespace, err := createNamespace(ctx, c, logger)
	if err != nil {
		return err
	}
	operator, proxyTkn, natsCfg, err := createOperator(ctx, c, logger, namespace.Id)
	if err != nil {
		return fmt.Errorf("create operator: %w", err)
	}
	logger.Info(fmt.Sprintf("created operator '%s'", operator.Name))

	ns, err := startNATS(natsCfg)
	if err != nil {
		return fmt.Errorf("start nats: %w", err)
	}
	logger.Info(fmt.Sprintf("started nats server on %s", ns.ClientURL()))
	defer ns.Shutdown()

	brokerURL := wsURL + "/broker"
	proxy, err := startProxy(ctx, c, logger, ns.ClientURL(), brokerURL, namespace.Id, operator.Id, proxyTkn)
	if err != nil {
		return err
	}
	defer proxy.Stop()

	account, err := createAccount(ctx, c, logger, namespace.Id, operator.Id)
	if err != nil {
		return err
	}

	user, userJWT, userSeed, err := createUser(ctx, c, logger, namespace.Id, account.Id)
	if err != nil {
		return err
	}

	nc, js, err := connectNATS(ns, logger, user, userJWT, userSeed)
	if err != nil {
		return err
	}
	defer nc.Close()

	if err = createStreamAndPublisher(ctx, js, logger, errCh); err != nil {
		return err
	}

	logger.Info("dev server ready")
	logger.Info(fmt.Sprintf("api: http://127.0.0.1:%d/api/v1", cfg.port))

	if cfg.enableUI {
		logger.Info(fmt.Sprintf("ui: http://127.0.0.1:%d", cfg.port))
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}
