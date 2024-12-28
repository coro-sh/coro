package entity

import "go.jetify.com/typeid"

type namespacePrefix struct{}

func (namespacePrefix) Prefix() string { return "ns" }

// NamespaceID is the unique identifier for a Namespace.
type NamespaceID struct {
	typeid.TypeID[namespacePrefix]
}

type Namespace struct {
	ID   NamespaceID `json:"id"`
	Name string      `json:"name"`
}

// NewNamespace creates a new Namespace.
func NewNamespace(name string) *Namespace {
	return &Namespace{
		ID:   NewID[NamespaceID](),
		Name: name,
	}
}
