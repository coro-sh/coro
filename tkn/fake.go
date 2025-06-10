package tkn

import (
	"context"
	"errors"
	"testing"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/testutil"
)

var errFakeNotFound = errtag.NewTagged[errtag.NotFound]("not found")

var _ OperatorTokenReadWriter = (*FakeOperatorTokenReadWriter)(nil)

type FakeOperatorTokenReadWriter struct {
	tokenIndexes *testutil.KV[OperatorTokenType, *testutil.KV[entity.OperatorID, string]]
}

func NewFakeOperatorTokenReadWriter(t *testing.T) *FakeOperatorTokenReadWriter {
	t.Helper()
	indexes := testutil.NewKV[OperatorTokenType, *testutil.KV[entity.OperatorID, string]](t)
	indexes.Put(OperatorTokenTypeProxy, testutil.NewKV[entity.OperatorID, string](t))
	return &FakeOperatorTokenReadWriter{
		tokenIndexes: indexes,
	}
}

func (rw *FakeOperatorTokenReadWriter) Read(_ context.Context, tokenType OperatorTokenType, operatorID entity.OperatorID) (string, error) {
	index, ok := rw.tokenIndexes.Get(tokenType)
	if !ok {
		return "", errors.New("index not found")
	}
	tkn, ok := index.Get(operatorID)
	if !ok {
		return "", errtag.Tag[errtag.NotFound](errFakeNotFound)
	}
	return tkn, nil
}

func (rw *FakeOperatorTokenReadWriter) Write(_ context.Context, tokenType OperatorTokenType, operatorID entity.OperatorID, hashedToken string) error {
	index, ok := rw.tokenIndexes.Get(tokenType)
	if !ok {
		return errors.New("index not found")
	}
	index.Put(operatorID, hashedToken)
	return nil
}
