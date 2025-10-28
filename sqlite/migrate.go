package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file" // register the file source driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
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

func MigrateDatabase(db *sql.DB, fsys fs.FS, opts ...MigrateOption) error {
	var mopts migrationOptions
	for _, opt := range opts {
		opt(&mopts)
	}

	sd, err := iofs.New(fsys, ".")
	if err != nil {
		return fmt.Errorf("open migrations fs: %w", err)
	}
	defer sd.Close() //nolint:errcheck

	driver, err := sqlite.WithInstance(db, new(sqlite.Config))
	if err != nil {
		return fmt.Errorf("create sqlite driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sd, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
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
