package ast

import (
	"fmt"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/types"
)

// TypeResolver defines the interface for resolving type information.
type TypeResolver interface {
	Resolve(typeName string) (types.TypeInfo, error)
	UpdateFromWalker(walker *PackageWalker) error
	GetImports() map[string]string
	GetKnownTypes() map[string]types.TypeInfo
}

// TypeResolverImpl implements the TypeResolver interface.
type TypeResolverImpl struct {
	Pkgs      []*packages.Package
	walker    *PackageWalker
	typeCache map[string]types.TypeInfo
	imports   map[string]string
}

// NewResolver creates a new TypeResolver.
func NewResolver(pkgs []*packages.Package) TypeResolver {
	return &TypeResolverImpl{
		Pkgs:      pkgs,
		typeCache: make(map[string]types.TypeInfo),
		imports:   make(map[string]string),
	}
}

// Resolve resolves a type name to its TypeInfo.
func (r *TypeResolverImpl) Resolve(typeName string) (types.TypeInfo, error) {
	if r.walker == nil {
		return types.TypeInfo{}, fmt.Errorf("resolver has not been updated with a walker")
	}
	return r.walker.Resolve(typeName)
}

// UpdateFromWalker updates the resolver with information from a PackageWalker.
func (r *TypeResolverImpl) UpdateFromWalker(walker *PackageWalker) error {
	r.walker = walker
	return nil
}

// GetImports returns the collected imports.
func (r *TypeResolverImpl) GetImports() map[string]string {
	if r.walker == nil {
		return make(map[string]string)
	}
	return r.walker.GetImports()
}

// GetKnownTypes returns the cache of known types.
func (r *TypeResolverImpl) GetKnownTypes() map[string]types.TypeInfo {
	if r.walker == nil {
		return make(map[string]types.TypeInfo)
	}
	return r.walker.GetTypeCache()
}
