package id

import "go.jetify.com/typeid"

type ID interface {
	typeid.Subtype
	IsZero() bool
}

// New creates a new instance of the specified ID type. It panics if the ID
// cannot be generated.
func New[T ID, PI typeid.SubtypePtr[T]]() T {
	return typeid.Must(typeid.New[T, PI]())
}

// Parse parses a string representation of an ID into the specified ID type.
func Parse[I ID, PI typeid.SubtypePtr[I]](id string) (I, error) {
	return typeid.Parse[I, PI](id)
}

// MustParse parses a string representation of an ID into the specified ID
// type and panics if it cannot be parsed.
func MustParse[I ID, PI typeid.SubtypePtr[I]](id string) I {
	return typeid.Must(Parse[I, PI](id))
}
