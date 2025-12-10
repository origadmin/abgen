package analyzer

import (
	"fmt"
	"go/types"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

var _ *TypeInfo // Dummy reference to ensure TypeInfo is recognized within the package

const abgenDirectivePrefix = "//go:abgen:"

// PackageWalker is responsible for walking through Go packages,
// resolving types, and building a collection of TypeInfo structures.
type PackageWalker struct {
	pkgs        []*packages.Package
	typeCache   map[types.Type]*TypeInfo
	failedLoads map[string]bool // Tracks packages that failed to load to prevent infinite retries
}

// NewPackageWalker creates a new PackageWalker.
func NewPackageWalker() *PackageWalker {
	// Temporarily set slog to Debug level for testing
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return &PackageWalker{
		typeCache:   make(map[types.Type]*TypeInfo),
		failedLoads: make(map[string]bool),
	}
}

// LoadInitialPackage loads only the essential information for a single package
// in order to discover directives. It does not perform full type checking.
func (w *PackageWalker) LoadInitialPackage(path string) (*packages.Package, error) {
	cfg := &packages.Config{
		Mode:       packages.NeedSyntax | packages.NeedFiles,
		Dir:        path,
		Tests:      false,
		BuildFlags: []string{"-tags=abgen"},
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to load initial package: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("initial package contains errors")
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no initial package found at path: %s", path)
	}
	return pkgs[0], nil
}

// DiscoverDirectives and ExtractDependencies have been moved to config package.
// The PackageWalker now only focuses on type resolution.

// LoadFullGraph loads the complete type information for the initial package
// and all its specified dependencies.
func (w *PackageWalker) LoadFullGraph(initialPath string, dependencyPaths ...string) ([]*packages.Package, error) {
	loadPatterns := append([]string{initialPath}, dependencyPaths...)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir:        ".",
		Tests:      false,
		BuildFlags: []string{"-tags=abgen"},
	}

	pkgs, err := packages.Load(cfg, loadPatterns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load full package graph: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("full package graph contains errors")
	}

	w.pkgs = pkgs
	return pkgs, nil
}

// FindTypeByFQN is the main entry point for resolving a type.
// It can dynamically load missing packages if needed.
func (w *PackageWalker) FindTypeByFQN(fqn string) (*TypeInfo, error) {
	slog.Debug("FindTypeByFQN called", "fqn", fqn) // Added log

	// First, try to find in already loaded packages
	if info, err := w.findTypeInLoadedPkgs(fqn); err == nil && info != nil {
		slog.Debug("FindTypeByFQN found in loaded packages", "fqn", fqn) // Added log
		return info, nil
	}

	// If not found, extract package path and try to load the missing package
	pkgPath := fqn[:strings.LastIndex(fqn, ".")]
	if pkgPath == "" {
		return nil, fmt.Errorf("invalid FQN: %s", fqn)
	}

	// Check if we already tried and failed to load this package
	if w.failedLoads[pkgPath] {
		slog.Debug("FindTypeByFQN skipping failed package", "pkgPath", pkgPath) // Added log
		return nil, fmt.Errorf("package %q previously failed to load", pkgPath)
	}

	// Try to load the missing package dynamically
	if err := w.loadMissingPackage(pkgPath); err != nil {
		w.failedLoads[pkgPath] = true
		slog.Error("FindTypeByFQN failed to load missing package", "pkgPath", pkgPath, "fqn", fqn, "error", err) // Added log
		return nil, fmt.Errorf("failed to load missing package %q for type %q: %w", pkgPath, fqn, err)
	}

	// Now that the package is loaded, try to find the type again
	if info, err := w.findTypeInLoadedPkgs(fqn); err == nil && info != nil {
		slog.Debug("FindTypeByFQN found after dynamic load", "fqn", fqn) // Added log
		return info, nil
	}

	slog.Debug("FindTypeByFQN type not found after all attempts", "fqn", fqn) // Added log
	return nil, fmt.Errorf("type %q not found after loading package %q", fqn, pkgPath)
}

// findTypeInLoadedPkgs searches for a type in all already loaded packages.
func (w *PackageWalker) findTypeInLoadedPkgs(fqn string) (*TypeInfo, error) {
	pkgPath := fqn[:strings.LastIndex(fqn, ".")]
	typeName := fqn[strings.LastIndex(fqn, ".")+1:]

	if pkgPath == "" || typeName == "" {
		return nil, fmt.Errorf("invalid FQN: %s", fqn)
	}

	slog.Debug("Searching for type in loaded packages", "fqn", fqn, "pkgPath", pkgPath, "typeName", typeName) // Added log

	for _, pkg := range w.pkgs {
		slog.Debug("Checking package", "currentPkgPath", pkg.PkgPath, "targetPkgPath", pkgPath) // Added log
		if pkg.PkgPath == pkgPath {
			slog.Debug("Found matching package path", "pkgPath", pkg.PkgPath) // Added log
			slog.Debug("Package scope names", "names", pkg.Types.Scope().Names()) // Added log

			obj := pkg.Types.Scope().Lookup(typeName)
			if obj == nil {
				slog.Debug("Object not found in package scope", "typeName", typeName, "pkgPath", pkg.PkgPath) // Added log
				continue
			}

			slog.Debug("Object found in package scope", "typeName", typeName, "pkgPath", pkg.PkgPath, "obj", obj) // Added log

			// Check if the found object is a TypeName.
			tn, ok := obj.(*types.TypeName)
			if !ok {
				slog.Debug("Object is not a TypeName", "typeName", typeName, "objType", fmt.Sprintf("%T", obj)) // Added log
				return nil, fmt.Errorf("%q is not a type name", fqn)
			}

			// If it's an alias, handle it here at the top level.
			if tn.IsAlias() {
				slog.Debug("Found alias type", "fqn", fqn) // Added log
				info := &TypeInfo{
					Name:       tn.Name(),
					ImportPath: tn.Pkg().Path(),
					IsAlias:    true,
					Original:   tn,
				}
				// The underlying type of an alias is the type it points to.
				// For 'type A = B', tn.Type() returns *types.Alias.
				// We need to unwrap that *types.Alias to get B.
				resolvedUnderlyingType := tn.Type()
				if aliasType, ok := resolvedUnderlyingType.(*types.Alias); ok {
					resolvedUnderlyingType = aliasType.Underlying()
				}
				underlyingInfo := w.resolveType(resolvedUnderlyingType)

				// For aliases, we want to inherit the full structure of the underlying type.
				// This includes its Underlying, Kind, KeyType, etc.
				// For aliases, we want to point to the element type for composite types
				// This ensures proper type hierarchy for GoTypeString
				if elem := getElementOrValueType(underlyingInfo); elem != nil {
					info.Underlying = elem
				} else {
					info.Underlying = underlyingInfo
				}
				info.Kind = underlyingInfo.Kind
				info.KeyType = underlyingInfo.KeyType
				info.ArrayLen = underlyingInfo.ArrayLen
				info.Fields = underlyingInfo.Fields

				return info, nil
			}

			// If it's a regular type definition, resolve it normally.
			slog.Debug("Found regular type definition", "fqn", fqn) // Added log
			return w.resolveType(obj.Type()), nil
		}
	}
	slog.Debug("Type not found in any loaded package", "fqn", fqn) // Added log
	return nil, fmt.Errorf("type %q not found in loaded packages", fqn)
}

// loadMissingPackage dynamically loads a package that hasn't been loaded yet.
func (w *PackageWalker) loadMissingPackage(pkgPath string) error {
	// Check if already loaded
	for _, pkg := range w.pkgs {
		if pkg.PkgPath == pkgPath {
			return nil
		}
	}

	slog.Debug("Dynamically loading missing package", "path", pkgPath)

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Tests:      false,
		BuildFlags: []string{"-tags=abgen"},
	}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		w.failedLoads[pkgPath] = true
		return fmt.Errorf("failed to load package %q: %w", pkgPath, err)
	}

	if len(pkgs) == 0 {
		w.failedLoads[pkgPath] = true
		return fmt.Errorf("no package found for import path %q", pkgPath)
	}

	errorCount := packages.PrintErrors(pkgs)
	if errorCount > 0 {
		slog.Warn("Errors while loading package", "path", pkgPath, "errorCount", errorCount)
		// If there are errors, consider the package failed to load
		w.failedLoads[pkgPath] = true
		return fmt.Errorf("package %q loaded with %d errors", pkgPath, errorCount)
	}

	// Add the loaded package to our package list
	w.pkgs = append(w.pkgs, pkgs[0])

	slog.Debug("Successfully loaded missing package", "path", pkgPath, "files", len(pkgs[0].Syntax))
	return nil
}

// GetLoadedPackages returns the list of loaded packages
func (w *PackageWalker) GetLoadedPackages() []*packages.Package {
	return w.pkgs
}

// GetFailedPackagesCount returns the count of failed package loads
func (w *PackageWalker) GetFailedPackagesCount() int {
	return len(w.failedLoads)
}

// Helper function to get the "element type" from a TypeInfo if it's a composite
func getElementOrValueType(ti *TypeInfo) *TypeInfo {
	if ti == nil {
		slog.Debug("getElementOrValueType", "input_ti", "nil", "result", "nil")
		return nil
	}
	switch ti.Kind {
	case Pointer, Slice, Array:
		slog.Debug("getElementOrValueType", "input_ti", ti.Name, "input_kind", ti.Kind, "result_ti", ti.Underlying)
		return ti.Underlying // This is the element type
	case Map:
		slog.Debug("getElementOrValueType", "input_ti", ti.Name, "input_kind", ti.Kind, "result_ti", ti.Underlying)
		return ti.Underlying // This is the value type
	default:
		slog.Debug("getElementOrValueType", "input_ti", ti.Name, "input_kind", ti.Kind, "result_ti", "nil (not composite)")
		return nil // Not a composite with a direct element
	}
}

// resolveType is the recursive worker for building the TypeInfo graph.

// It should not be called directly for top-level types that might be aliases.

func (w *PackageWalker) resolveType(typ types.Type) *TypeInfo {

	if typ == nil {

		return nil

	}

	if cached, exists := w.typeCache[typ]; exists {

		slog.Debug("resolveType (cached)", "typ", typ.String(), "info", cached)

		return cached

	}

	info := &TypeInfo{}

	// IMPORTANT: Add to cache immediately to prevent infinite recursion for self-referencing types
	w.typeCache[typ] = info 
	
	slog.Debug("resolveType (new)", "typ", typ.String())

	switch t := typ.(type) {

	case *types.Alias: // Handles 'type T = U' declarations

		obj := t.Obj()

		info.Name = obj.Name()

		if obj.Pkg() != nil {

			info.ImportPath = obj.Pkg().Path()

		}

		info.IsAlias = true

		info.Original = obj

		underlyingType := t.Underlying()

		// Directly unwrap composite underlying types for aliases

		if ptr, ok := underlyingType.(*types.Pointer); ok {

			info.Kind = Pointer

			info.Underlying = w.resolveType(ptr.Elem()) // Alias points to element of the pointer

		} else if slice, ok := underlyingType.(*types.Slice); ok {

			info.Kind = Slice

			info.Underlying = w.resolveType(slice.Elem()) // Alias points to element of the slice

		} else if array, ok := underlyingType.(*types.Array); ok {

			info.Kind = Array

			info.ArrayLen = int(array.Len())

			info.Underlying = w.resolveType(array.Elem()) // Alias points to element of the array

		} else if mp, ok := underlyingType.(*types.Map); ok {

			info.Kind = Map

			info.KeyType = w.resolveType(mp.Key())

			info.Underlying = w.resolveType(mp.Elem()) // Alias points to value of the map

		} else {

			// Not a composite underlying type (e.g., alias to struct, basic)

			underlyingInfo := w.resolveType(underlyingType)

			info.Kind = underlyingInfo.Kind

			info.Underlying = underlyingInfo

			info.KeyType = underlyingInfo.KeyType

			info.ArrayLen = underlyingInfo.ArrayLen

			info.Fields = underlyingInfo.Fields

		}

		slog.Debug("resolveType (*types.Alias)", "typ", typ.String(), "info", info)

	case *types.Named: // Handles 'type T U' declarations

		slog.Debug("resolveType (*types.Named)", "typ", typ.String(), "obj.Name", t.Obj().Name(), "t.Underlying() type", fmt.Sprintf("%T", t.Underlying()), "t.Underlying() value", t.Underlying().String())

		obj := t.Obj()

		info.Name = obj.Name()

		if obj.Pkg() != nil {

			info.ImportPath = obj.Pkg().Path()

		}

		info.IsAlias = false // Explicitly false for type definitions

		info.Original = obj

		underlyingInfo := w.resolveType(t.Underlying()) // This is TypeInfo(U)

		info.Kind = underlyingInfo.Kind

		// For defined types, Underlying points to the resolved element/value type of U (if U is composite),

		// or to U itself if U is a struct/primitive/etc.

		if elem := getElementOrValueType(underlyingInfo); elem != nil {

			info.Underlying = elem

		} else {

			info.Underlying = underlyingInfo

		}

		info.KeyType = underlyingInfo.KeyType

		info.ArrayLen = underlyingInfo.ArrayLen

		info.Fields = underlyingInfo.Fields

		slog.Debug("resolveType (*types.Named)", "typ", typ.String(), "info", info)

	case *types.Pointer:

		info.Kind = Pointer

		info.Underlying = w.resolveType(t.Elem())

		// info.Name is intentionally left empty for anonymous composite types

		slog.Debug("resolveType (*types.Pointer)", "typ", typ.String(), "info", info)

	case *types.Slice:

		info.Kind = Slice

		info.Underlying = w.resolveType(t.Elem())

		// info.Name is intentionally left empty for anonymous composite types

		slog.Debug("resolveType (*types.Slice)", "typ", typ.String(), "info", info)

	case *types.Array:

		info.Kind = Array

		info.ArrayLen = int(t.Len())

		info.Underlying = w.resolveType(t.Elem())

		// info.Name is intentionally left empty for anonymous composite types

		slog.Debug("resolveType (*types.Array)", "typ", typ.String(), "info", info)

	case *types.Map:

		info.Kind = Map

		info.KeyType = w.resolveType(t.Key())

		info.Underlying = w.resolveType(t.Elem()) // Value type

		// info.Name is intentionally left empty for anonymous composite types

		slog.Debug("resolveType (*types.Map)", "typ", typ.String(), "info", info)

	case *types.Struct:

		info.Kind = Struct

		info.Fields = w.parseFields(t)

		slog.Debug("resolveType (*types.Struct)", "typ", typ.String(), "info", info)

	case *types.Basic:

		info.Kind = Primitive

		info.Name = t.Name()

		slog.Debug("resolveType (*types.Basic)", "typ", typ.String(), "info", info)

	case *types.Interface:

		info.Kind = Interface

		slog.Debug("resolveType (*types.Interface)", "typ", typ.String(), "info", info)

	case *types.Chan:

		info.Kind = Chan

		slog.Debug("resolveType (*types.Chan)", "typ", typ.String(), "info", info)

	case *types.Signature:

		info.Kind = Func

		slog.Debug("resolveType (*types.Signature)", "typ", typ.String(), "info", info)

	default:

		slog.Warn("unhandled type in resolveType", "type", fmt.Sprintf("%T", t), "value", t.String())

		info.Kind = Unknown

		info.Name = t.String()

		slog.Debug("resolveType (default)", "typ", typ.String(), "info", info)

	}

	// No need to insert into cache again here, it's already there
	return info

}

// parseFields parses the fields of a *types.Struct.
func (w *PackageWalker) parseFields(s *types.Struct) []*FieldInfo {
	fields := make([]*FieldInfo, 0, s.NumFields())
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if !f.Exported() {
			continue
		}
		fields = append(fields, &FieldInfo{
			Name:       f.Name(),
			Type:       w.resolveType(f.Type()),
			Tag:        s.Tag(i),
			IsEmbedded: f.Embedded(),
		})
	}
	return fields
}
