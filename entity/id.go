package entity

import "go.jetify.com/typeid"

// ID is a generic identifier interface that combines all entity ID types
// (NamespaceID, OperatorID, AccountID, UserID).
type ID interface {
	NamespaceID | OperatorID | AccountID | UserID
	typeid.Subtype
	IsZero() bool
}

// NewID creates a new instance of the specified ID type. It panics if the ID
// cannot be generated.
func NewID[T ID, PT typeid.SubtypePtr[T]]() T {
	return typeid.Must(typeid.New[T, PT]())
}

// ParseID parses a string representation of an ID into the specified ID type.
func ParseID[T ID, PT typeid.SubtypePtr[T]](id string) (T, error) {
	return typeid.Parse[T, PT](id)
}

// MustParseID parses a string representation of an ID into the specified ID
// type and panics if it cannot be parsed.
func MustParseID[T ID, PT typeid.SubtypePtr[T]](id string) T {
	return typeid.Must(ParseID[T, PT](id))
}
