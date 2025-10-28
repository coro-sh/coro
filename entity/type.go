package entity

import "fmt"

// Type represents the entity type.
type Type int

const (
	TypeUnspecified Type = iota
	TypeOperator
	TypeAccount
	TypeUser
	typeEnd
)

var typeNames = map[Type]string{
	TypeUnspecified: "unspecified",
	TypeOperator:    "operator",
	TypeAccount:     "account",
	TypeUser:        "user",
}

var nameTypes = map[string]Type{
	"unspecified": TypeUnspecified,
	"operator":    TypeOperator,
	"account":     TypeAccount,
	"user":        TypeUser,
}

// String returns the string representation of the Type.
// Defaults to "unspecified" if unrecognized.
func (t Type) String() string {
	if name, ok := typeNames[t]; ok {
		return name
	}
	return typeNames[TypeUnspecified]
}

type Entity interface {
	Namespace | Operator | Account | User
}

func GetTypeName[T Entity]() string {
	var t T
	switch v := any(t).(type) {
	case Namespace:
		return "namespace"
	case Operator:
		return "operator"
	case Account:
		return "account"
	case User:
		return "user"
	default:
		panic(fmt.Sprintf("failed to get entity type name: unknown entity type %T", v))
	}
}

// GetTypeNameFromID returns the type name of the ID.
func GetTypeNameFromID[T ID]() string {
	var t T
	switch v := any(t).(type) {
	case NamespaceID:
		return "namespace"
	case OperatorID:
		return "operator"
	case AccountID:
		return "account"
	case UserID:
		return "user"
	default:
		panic(fmt.Sprintf("failed to get entity type name from id: unknown id type %T", v))
	}
}

func ParseTypeFromString(typeName string) Type {
	if t, ok := nameTypes[typeName]; ok {
		return t
	}
	return TypeUnspecified
}
