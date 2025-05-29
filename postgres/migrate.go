package postgres

import (
	"errors"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // register the file source driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

type migrationOptions struct {
	version *uint
}

type MigrateOption func(opts *migrationOptions)

func WithMigrationVersion(version uint) MigrateOption {
	return func(opts *migrationOptions) {
		opts.version = &version
	}
}

func MigrateDatabase(pool *pgxpool.Pool, fsys fs.FS, opts ...MigrateOption) error {
	var mopts migrationOptions
	for _, opt := range opts {
		opt(&mopts)
	}

	sd, err := iofs.New(fsys, ".")
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
