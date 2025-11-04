// Package constants defines a collection of names and keywords that have
// special meanings within the context of the app. These values are used to
// enforce naming constraints, avoid conflicts, and ensure consistency across
// various components of the system. This package centralizes these constants to
// maintain a single source of truth for all identifiers, making it easier to
// update and reference them throughout the codebase.
package constants

const (
	AppName                = "coro"
	AppNameUpper           = "CORO"
	SysAccountName         = "SYS"
	SysUserName            = "sys"
	BrokerOperatorName     = AppName + "_broker"
	InternalNamespaceName  = AppName + "_internal"
	InternalNamespaceOwner = AppName
	DefaultNamespaceName   = "default"
	DefaultNamespaceOwner  = "default"
)

func IsReservedNamespaceName(name string) bool {
	return name == InternalNamespaceName
}

func IsReservedOperatorName(name string) bool {
	return name == BrokerOperatorName
}

func IsReservedAccountName(name string) bool {
	return name == SysAccountName
}

func IsReservedUserName(name string) bool {
	return name == SysUserName
}
