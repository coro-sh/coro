package entity

import (
	"github.com/joshjon/kit/id"
	"go.jetify.com/typeid"
)

type namespacePrefix struct{}

func (namespacePrefix) Prefix() string { return "ns" }

// NamespaceID is the unique identifier for a Namespace.
type NamespaceID struct {
	typeid.TypeID[namespacePrefix]
}

type Namespace struct {
	ID    NamespaceID `json:"id"`
	Name  string      `json:"name"`
	Owner string      `json:"-"`
}

// NewNamespace creates a new Namespace.
func NewNamespace(name string, owner string) *Namespace {
	return &Namespace{
		ID:    id.New[NamespaceID](),
		Name:  name,
		Owner: owner,
	}
}
