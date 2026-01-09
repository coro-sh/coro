package entity

import (
	kid "github.com/joshjon/kit/id"
	"go.jetify.com/typeid"
)

// ID is a generic identifier interface that combines all entity ID types
// (NamespaceID, OperatorID, AccountID, UserID).
type ID interface {
	NamespaceID | OperatorID | AccountID | UserID
	typeid.Subtype
	IsZero() bool
}

// NewID creates a new instance of the specified ID type. It panics if the ID
// cannot be generated.
func NewID[I ID, PI typeid.SubtypePtr[I]]() I {
	return kid.New[I, PI]()
}

// ParseID parses a string representation of an ID into the specified ID type.
func ParseID[I ID, PI typeid.SubtypePtr[I]](id string) (I, error) {
	return kid.Parse[I, PI](id)
}

// MustParseID parses a string representation of an ID into the specified ID
// type and panics if it cannot be parsed.
func MustParseID[I ID, PI typeid.SubtypePtr[I]](id string) I {
	return kid.MustParse[I, PI](id)
}
