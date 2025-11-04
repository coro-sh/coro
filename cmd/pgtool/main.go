package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/cohesivestack/valgo"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/urfave/cli/v2"

	"github.com/coro-sh/coro/log"
	"github.com/coro-sh/coro/postgres"
	"github.com/coro-sh/coro/postgres/migrations"
)

const (
	defaultPort = 5432
	defaultDB   = "postgres"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	app := cli.NewApp()
	app.Name = "coro-pgtool"
	app.Usage = "Command line tool to manage the Coro Postgres database"

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Aliases: []string{"ho"},
			Value:   "127.0.0.1",
			Usage:   "[required] hostname or ip address of postgres",
			EnvVars: []string{"POSTGRES_HOST"},
		},
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   defaultPort,
			Usage:   "[required] port of postgres",
			EnvVars: []string{"POSTGRES_PORT"},
		},
		&cli.StringFlag{
			Name:    "user",
			Aliases: []string{"u"},
			Value:   "",
			Usage:   "[required] username for auth when connecting to postgres",
			EnvVars: []string{"POSTGRES_USER"},
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"pw"},
			Value:   "",
			Usage:   "[required] password for auth when connecting to postgres",
			EnvVars: []string{"POSTGRES_PASSWORD"},
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "create",
			Usage: "creates the database",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "default-db",
					Value: defaultDB,
					Usage: "default db to connect to (not the coro db to be created)",
				},
			},
			Action: execCmd(cmdCreateDB),
		},
		{
			Name:  "drop",
			Usage: "drops the database",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "default-db",
					Aliases: []string{"d"},
					Value:   defaultDB,
					Usage:   "default db to connect to (not the coro db to be dropped)",
				},
			},
			Action: execCmd(cmdDropDB),
		},
		{
			Name:   "migrate",
			Usage:  "applies all pending database schema migrations",
			Action: execCmd(cmdMigrate),
		},
		{
			Name:  "migrate-version",
			Usage: "migrates the database to a specific schema version",
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:    "version",
					Aliases: []string{"v"},
					Usage:   "desired schema version",
				},
			},
			Action: execCmd(cmdMigrateVersion),
		},
		{
			Name:  "init",
			Usage: "creates the database and migrates to the latest schema version",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "default-db",
					Value: defaultDB,
					Usage: "default db to connect to (not the coro db to be created)",
				},
			},
			Action: execCmd(cmdInit),
		},
	}

	return app.Run(args)
}

func cmdCreateDB(ctx context.Context, cfg config, c *cli.Context, l log.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	database := c.String("default-db")
	exitOnInvalidFlags(c, valgo.Is(valgo.String(database, "default-db").Not().Blank()))

	l.Info("connecting to default database", "database", database)
	hostPort := fmt.Sprintf("%s:%d", cfg.host, cfg.port)
	conn, err := postgres.Dial(ctx, cfg.user, cfg.password, hostPort, database)
	if err != nil {
		return err
	}
	defer conn.Close()

	l = l.With("database", postgres.AppDBName)

	l.Info("creating database")
	if _, err = conn.Exec(ctx, "CREATE DATABASE "+postgres.AppDBName); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code != pgerrcode.DuplicateDatabase {
				return err
			}
			l.Info("database already exists")
		}
	}
	l.Info("database successfully created")

	return nil
}

func cmdDropDB(ctx context.Context, cfg config, c *cli.Context, l log.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	database := c.String("default-db")
	exitOnInvalidFlags(c, valgo.Is(valgo.String(database, "default-db").Not().Blank()))

	l.Info("connecting to default database", "database", database)
	hostPort := fmt.Sprintf("%s:%d", cfg.host, cfg.port)
	conn, err := postgres.Dial(ctx, cfg.user, cfg.password, hostPort, defaultDB)
	if err != nil {
		return err
	}
	defer conn.Close()

	l = l.With("database", postgres.AppDBName)

	l.Info("dropping database")
	if _, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+postgres.AppDBName); err != nil {
		return err
	}
	l.Info("database successfully dropped", "database")

	return nil
}

func cmdMigrate(ctx context.Context, cfg config, _ *cli.Context, l log.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	l = l.With("database", postgres.AppDBName)

	l.Info("connecting to database")
	hostPort := fmt.Sprintf("%s:%d", cfg.host, cfg.port)
	conn, err := postgres.Dial(ctx, cfg.user, cfg.password, hostPort, postgres.AppDBName)
	if err != nil {
		return err
	}
	defer conn.Close()

	l.Info("migrating database")
	if err = postgres.MigrateDatabase(conn, migrations.FS); err != nil {
		return err
	}
	l.Info("successfully migrated database")

	return nil
}

func cmdMigrateVersion(ctx context.Context, cfg config, c *cli.Context, l log.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	version := c.Uint("version")
	exitOnInvalidFlags(c, valgo.Is(valgo.Uint64(uint64(version), "version").GreaterThan(0)))

	l = l.With("database", postgres.AppDBName)

	l.Info("connecting to database")
	hostPort := fmt.Sprintf("%s:%d", cfg.host, cfg.port)
	conn, err := postgres.Dial(ctx, cfg.user, cfg.password, hostPort, postgres.AppDBName)
	if err != nil {
		return err
	}
	defer conn.Close()

	l = l.With("version", version)
	l.Info("migrating database")
	if err = postgres.MigrateDatabase(conn, migrations.FS, postgres.WithMigrationVersion(version)); err != nil {
		return err
	}
	l.Info("successfully migrated database")

	return nil
}

func cmdInit(ctx context.Context, cfg config, c *cli.Context, l log.Logger) error {
	if err := cmdCreateDB(ctx, cfg, c, l); err != nil {
		return err
	}
	if err := cmdMigrate(ctx, cfg, c, l); err != nil {
		return err
	}
	return nil
}

func execCmd(cmd func(ctx context.Context, cfg config, c *cli.Context, l log.Logger) error) func(c *cli.Context) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	l := log.NewLogger()
	return func(c *cli.Context) error {
		defer cancel()
		cfg := loadConfig(c)
		return cmd(ctx, cfg, c, l)
	}
}

type config struct {
	host     string
	port     int
	user     string
	password string
}

func (c config) validate() *valgo.Validation {
	return valgo.Is(
		valgo.String(c.host, "host").Not().Blank(),
		valgo.Int(c.port, "port").GreaterThan(0),
		valgo.String(c.user, "user").Not().Blank(),
		valgo.String(c.password, "password").Not().Blank(),
	)
}

func loadConfig(c *cli.Context) config {
	cfg := config{
		host:     c.String("host"),
		port:     c.Int("port"),
		user:     c.String("user"),
		password: c.String("password"),
	}
	exitOnInvalidFlags(c, cfg.validate())
	return cfg
}

func exitOnInvalidFlags(c *cli.Context, v *valgo.Validation) {
	if v.Error() == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "Flag errors:")

	for _, verr := range v.Error().(*valgo.Error).Errors() {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", verr.Name(), strings.Join(verr.Messages(), ","))
	}

	fmt.Fprintln(os.Stdout) //nolint:errcheck
	cli.ShowAppHelpAndExit(c, 1)
}
