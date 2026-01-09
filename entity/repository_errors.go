package entity

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/joshjon/kit/errtag"
)

type ErrTagNkeyNotFound struct{ errtag.NotFound }

func (ErrTagNkeyNotFound) Msg() string { return "Nkey not found" }

func (e ErrTagNkeyNotFound) Unwrap() error {
	return errtag.Tag[errtag.NotFound](e.Cause())
}

type ErrTagSigningKeyNotFound struct{ errtag.NotFound }

func (ErrTagSigningKeyNotFound) Msg() string { return "Signing key not found" }

func (e ErrTagSigningKeyNotFound) Unwrap() error {
	return errtag.Tag[errtag.NotFound](e.Cause())
}

type ErrTagNkeyConflict struct{ errtag.Conflict }

func (ErrTagNkeyConflict) Msg() string { return "Nkey conflict" }

func (e ErrTagNkeyConflict) Unwrap() error {
	return errtag.Tag[errtag.Conflict](e.Cause())
}

type ErrTagSigningKeyConflict struct{ errtag.Conflict }

func (ErrTagSigningKeyConflict) Msg() string { return "Signing key conflict" }

func (e ErrTagSigningKeyConflict) Unwrap() error {
	return errtag.Tag[errtag.Conflict](e.Cause())
}

type ErrTagNotFound[T Entity] struct{ errtag.NotFound }

func (ErrTagNotFound[T]) Msg() string {
	return getTypeName[T]() + " not found"
}

func (e ErrTagNotFound[T]) Unwrap() error {
	return errtag.Tag[errtag.NotFound](e.Cause())
}

type ErrTagConflict[T Entity] struct{ errtag.Conflict }

func (ErrTagConflict[T]) Msg() string {
	return getTypeName[T]() + " conflict"
}

func (e ErrTagConflict[T]) Unwrap() error {
	return errtag.Tag[errtag.Conflict](e.Cause())
}

func getTypeName[T Entity]() string {
	typeName := GetTypeName[T]()
	caser := cases.Title(language.English)
	return caser.String(strings.ToLower(typeName))
}
