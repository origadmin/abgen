package ast

import (
	"fmt"
	"log/slog"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/types"
)

// TypeResolver defines the interface for resolving type information.
type TypeResolver interface {
	Resolve(typeName string) (*types.TypeInfo, error)
	UpdateFromWalker(walker *PackageWalker) error
	GetKnownTypes() map[string]*types.TypeInfo
	AddPackages(pkgs ...*packages.Package)
	GetLocalTypeNameToFQN() map[string]string
}

// TypeResolverImpl implements the TypeResolver interface.
type TypeResolverImpl struct {
	Pkgs    []*packages.Package
	walker  *PackageWalker
	imports map[string]string
}

// AddPackages adds more *packages.Package instances to the resolver's known packages.
func (r *TypeResolverImpl) AddPackages(newPkgs ...*packages.Package) {
	existingPkgs := make(map[string]bool)
	for _, p := range r.Pkgs {
		existingPkgs[p.PkgPath] = true
	}

	for _, newPkg := range newPkgs {
		if !existingPkgs[newPkg.PkgPath] {
			r.Pkgs = append(r.Pkgs, newPkg)
			existingPkgs[newPkg.PkgPath] = true
		}
	}
	// Also add packages to the internal walker's analyzer walker
	if r.walker != nil {
		r.walker.AddPackages(newPkgs...)
	}
}

// NewResolver creates a new TypeResolver.
func NewResolver(pkgs []*packages.Package) TypeResolver {
	return &TypeResolverImpl{
		Pkgs:    pkgs,
		imports: make(map[string]string),
	}
}

// Resolve resolves a type name to its TypeInfo.
func (r *TypeResolverImpl) Resolve(typeName string) (*types.TypeInfo, error) {
	if r.walker == nil {
		return nil, fmt.Errorf("resolver has not been updated with a walker")
	}
	info, err := r.walker.Resolve(typeName)
	if err == nil && info != nil {
		slog.Debug("Resolve: 成功解析类型", "输入", typeName, "输出名", info.Name, "包路径", info.ImportPath)
	}
	return info, err
}

// UpdateFromWalker updates the resolver with information from a PackageWalker.
func (r *TypeResolverImpl) UpdateFromWalker(walker *PackageWalker) error {
	r.walker = walker
	// Pass the resolver's accumulated packages to the walker
	r.walker.AddPackages(r.Pkgs...)
	return nil
}

// GetKnownTypes returns the cache of known types.
func (r *TypeResolverImpl) GetKnownTypes() map[string]*types.TypeInfo {
	if r.walker == nil {
		return make(map[string]*types.TypeInfo)
	}
	return r.walker.GetTypeCache()
}

// GetLocalTypeNameToFQN returns the local type name to FQN map collected by the walker.
func (r *TypeResolverImpl) GetLocalTypeNameToFQN() map[string]string {
	if r.walker == nil {
		return make(map[string]string)
	}
	return r.walker.GetLocalTypeNameToFQN()
}
