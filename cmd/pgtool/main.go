package main

import (
	"fmt"
	"os"

	"github.com/joshjon/kit/pgctl"

	"github.com/coro-sh/coro/postgres"
	"github.com/coro-sh/coro/postgres/migrations"
)

func main() {
	// Check for database name from environment variable
	dbName := os.Getenv("POSTGRES_DATABASE")
	if dbName == "" {
		dbName = postgres.AppDBName
	}

	r, err := pgctl.NewRunner(pgctl.RunnerConfig{
		DBName:     dbName,
		Migrations: migrations.FS,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = r.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
