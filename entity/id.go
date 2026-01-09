package entity

import (
	"github.com/joshjon/kit/id"
)

// ID is a generic identifier interface that combines all entity ID types
// (NamespaceID, OperatorID, AccountID, UserID).
type ID interface {
	NamespaceID | OperatorID | AccountID | UserID
	id.ID
}
