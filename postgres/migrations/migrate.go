package migrations

import (
	"embed"
	"errors"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // register the file source driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed *.sql
var fs embed.FS

type options struct {
	version *uint
}

type Option func(opts *options)

func WithVersion(version uint) Option {
	return func(opts *options) {
		opts.version = &version
	}
}

func MigrateDatabase(pool *pgxpool.Pool, opts ...Option) error {
	var mopts options
	for _, opt := range opts {
		opt(&mopts)
	}

	sd, err := iofs.New(fs, ".")
	if err != nil {
		return err
	}
	defer sd.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	driver, err := postgres.WithInstance(db, new(postgres.Config))
	if err != nil {
		return err
	}
	defer driver.Close()

	m, err := migrate.NewWithInstance("iofs", sd, "postgres", driver)
	if err != nil {
		return err
	}

	if mopts.version != nil {
		err = m.Migrate(*mopts.version)
	} else {
		err = m.Up()
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}
