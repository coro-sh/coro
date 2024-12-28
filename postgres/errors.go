package postgres

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
)

type NkeyNotFound struct{ errtag.NotFound }

func (NkeyNotFound) Msg() string { return "Nkey not found" }

func (e NkeyNotFound) Unwrap() error {
	return errtag.Tag[errtag.NotFound](e.Cause())
}

type SigningKeyNotFound struct{ errtag.NotFound }

func (SigningKeyNotFound) Msg() string { return "Signing key not found" }

func (e SigningKeyNotFound) Unwrap() error {
	return errtag.Tag[errtag.NotFound](e.Cause())
}

type EntityNotFound[T entity.Entity] struct{ errtag.NotFound }

func (EntityNotFound[T]) Msg() string {
	return getTypeName[T]() + " not found"
}

func (e EntityNotFound[T]) Unwrap() error {
	return errtag.Tag[errtag.NotFound](e.Cause())
}

type EntityConflict[T entity.Entity] struct{ errtag.Conflict }

func (EntityConflict[T]) Msg() string {
	return getTypeName[T]() + " conflict"
}

func (e EntityConflict[T]) Unwrap() error {
	return errtag.Tag[errtag.Conflict](e.Cause())
}

func getTypeName[T entity.Entity]() string {
	typeName := entity.GetTypeName[T]()
	caser := cases.Title(language.English)
	return caser.String(strings.ToLower(typeName))
}
