// Package interfaces defines the contracts between different components of the abgen system.
package interfaces

import (
	"github.com/origadmin/abgen/internal/analyzer"
)

// TypeResolver defines the contract for resolving type information.
type TypeResolver interface {
	// ResolveType resolves a type by its fully qualified name.
	ResolveType(fqn string) (*analyzer.TypeInfo, error)
}