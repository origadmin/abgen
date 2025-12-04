package ast

import (
	"fmt"
	"log/slog"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/types"
)

// TypeResolver defines the interface for resolving type information.
type TypeResolver interface {
	Resolve(typeName string) (types.TypeInfo, error)
	UpdateFromWalker(walker *PackageWalker) error
	GetImports() map[string]string
	GetKnownTypes() map[string]types.TypeInfo
	AddPackages(pkgs ...*packages.Package) // New method to add more packages dynamically
	GetLocalTypeAliases() map[string]string // New: Get local type aliases
}

// TypeResolverImpl implements the TypeResolver interface.
type TypeResolverImpl struct {
	Pkgs      []*packages.Package
	walker    *PackageWalker
	typeCache map[string]types.TypeInfo
	imports   map[string]string
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
	info, err := r.walker.Resolve(typeName)
	if err == nil {
		slog.Debug("Resolve: 成功解析类型", "输入", typeName, "输出名", info.Name, "包名", info.PkgName)
	}
	return info, err
}

// UpdateFromWalker updates the resolver with information from a PackageWalker.
func (r *TypeResolverImpl) UpdateFromWalker(walker *PackageWalker) error {
	r.walker = walker
	// Pass the resolver's accumulated packages to the walker
	r.walker.AddKnownPackages(r.Pkgs...)
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

// GetLocalTypeAliases returns the local type aliases collected by the walker.
func (r *TypeResolverImpl) GetLocalTypeAliases() map[string]string {
	if r.walker == nil {
		return make(map[string]string)
	}
	return r.walker.GetLocalTypeAliases()
}
