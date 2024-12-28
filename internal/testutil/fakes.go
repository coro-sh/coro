package testutil

import (
	"context"

	"github.com/coro-sh/coro/tx"
)

type FakeTx struct{}

func (t *FakeTx) Commit(_ context.Context) error { return nil }

func (t *FakeTx) Rollback(_ context.Context) error { return nil }

type FakeTxer struct{}

func (t *FakeTxer) BeginTx(_ context.Context) (tx.Tx, error) { return &FakeTx{}, nil }
