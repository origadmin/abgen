package analyzer

import (
	"fmt"
	"go/types"
	"log/slog"
	"strings"

	"golang.org/x/tools/go/packages"
)

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

// DiscoverDirectives scans the AST of a package and collects all abgen directives.
func (w *PackageWalker) DiscoverDirectives(pkg *packages.Package) []string {
	var directives []string
	for _, file := range pkg.Syntax {
		for _, commentGroup := range file.Comments {
			for _, comment := range commentGroup.List {
				if strings.HasPrefix(comment.Text, abgenDirectivePrefix) {
					directives = append(directives, comment.Text)
				}
			}
		}
	}
	return directives
}

// ExtractDependencies parses directives to find all referenced package paths.
func (w *PackageWalker) ExtractDependencies(directives []string) []string {
	depMap := make(map[string]struct{})
	for _, d := range directives {
		if strings.Contains(d, "path=") {
			parts := strings.Split(d, "path=")
			if len(parts) > 1 {
				pathPart := parts[1]
				path := strings.Split(pathPart, ",")[0]
				depMap[path] = struct{}{}
			}
		}
	}

	deps := make([]string, 0, len(depMap))
	for dep := range depMap {
		deps = append(deps, dep)
	}
	return deps
}

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
	// First, try to find in already loaded packages
	if info, err := w.findTypeInLoadedPkgs(fqn); err == nil && info != nil {
		return info, nil
	}

	// If not found, extract package path and try to load the missing package
	pkgPath := fqn[:strings.LastIndex(fqn, ".")]
	if pkgPath == "" {
		return nil, fmt.Errorf("invalid FQN: %s", fqn)
	}

	// Check if we already tried and failed to load this package
	if w.failedLoads[pkgPath] {
		return nil, fmt.Errorf("package %q previously failed to load", pkgPath)
	}

	// Try to load the missing package dynamically
	if err := w.loadMissingPackage(pkgPath); err != nil {
		w.failedLoads[pkgPath] = true
		return nil, fmt.Errorf("failed to load missing package %q for type %q: %w", pkgPath, fqn, err)
	}

	// Now that the package is loaded, try to find the type again
	if info, err := w.findTypeInLoadedPkgs(fqn); err == nil && info != nil {
		return info, nil
	}

	return nil, fmt.Errorf("type %q not found after loading package %q", fqn, pkgPath)
}

// findTypeInLoadedPkgs searches for a type in all already loaded packages.
func (w *PackageWalker) findTypeInLoadedPkgs(fqn string) (*TypeInfo, error) {
	pkgPath := fqn[:strings.LastIndex(fqn, ".")]
	typeName := fqn[strings.LastIndex(fqn, ".")+1:]

	if pkgPath == "" || typeName == "" {
		return nil, fmt.Errorf("invalid FQN: %s", fqn)
	}

	for _, pkg := range w.pkgs {
		if pkg.PkgPath == pkgPath {
			obj := pkg.Types.Scope().Lookup(typeName)
			if obj == nil {
				continue
			}

			// Check if the found object is a TypeName.
			tn, ok := obj.(*types.TypeName)
			if !ok {
				return nil, fmt.Errorf("%q is not a type name", fqn)
			}

			// If it's an alias, handle it here at the top level.
			if tn.IsAlias() {
				info := &TypeInfo{
					Name:       tn.Name(),
					ImportPath: tn.Pkg().Path(),
					IsAlias:    true,
					Original:   tn,
				}
				// The underlying type of an alias is the type it points to.
				info.Underlying = w.resolveType(tn.Type())
				// The Kind of an alias is determined by its underlying type.
				info.Kind = info.Underlying.Kind
				return info, nil
			}

			// If it's a regular type definition, resolve it normally.
			return w.resolveType(obj.Type()), nil
		}
	}
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

// resolveType is the recursive worker for building the TypeInfo graph.
// It should not be called directly for top-level types that might be aliases.
func (w *PackageWalker) resolveType(typ types.Type) *TypeInfo {
	if typ == nil {
		return nil
	}
	if cached, exists := w.typeCache[typ]; exists {
		return cached
	}

	info := &TypeInfo{}
	w.typeCache[typ] = info

	switch t := typ.(type) {
	case *types.Named:
		obj := t.Obj()
		info.Name = obj.Name()
		if obj.Pkg() != nil {
			info.ImportPath = obj.Pkg().Path()
		}
		info.IsAlias = obj.IsAlias()
		info.Original = obj

		underlyingType := t.Underlying()
		if info.IsAlias {
			// For aliases, the underlying type should be properly resolved
			info.Underlying = w.resolveType(underlyingType)
			info.Kind = info.Underlying.Kind
		} else {
			// For defined types, also resolve the underlying type
			info.Underlying = w.resolveType(underlyingType)
			info.Kind = info.Underlying.Kind
		}

		if s, ok := underlyingType.(*types.Struct); ok {
			info.Fields = w.parseFields(s)
		}

	case *types.Pointer:
		info.Kind = Pointer
		info.Underlying = w.resolveType(t.Elem())
		info.Name = "*" + info.Underlying.Name

	case *types.Slice:
		info.Kind = Slice
		info.Underlying = w.resolveType(t.Elem())
		info.Name = "[]" + info.Underlying.Name

	case *types.Array:
		info.Kind = Array
		info.ArrayLen = int(t.Len())
		info.Underlying = w.resolveType(t.Elem())
		info.Name = fmt.Sprintf("[%d]%s", info.ArrayLen, info.Underlying.Name)

	case *types.Map:
		info.Kind = Map
		info.KeyType = w.resolveType(t.Key())
		info.Underlying = w.resolveType(t.Elem()) // Value type
		info.Name = fmt.Sprintf("map[%s]%s", info.KeyType.Name, info.Underlying.Name)

	case *types.Struct:
		info.Kind = Struct
		info.Fields = w.parseFields(t)

	case *types.Basic:
		info.Kind = Primitive
		info.Name = t.Name()

	case *types.Interface:
		info.Kind = Interface

	case *types.Chan:
		info.Kind = Chan

	case *types.Signature:
		info.Kind = Func

	default:
		slog.Warn("unhandled type in resolveType", "type", t.String())
		info.Kind = Unknown
		info.Name = t.String()
	}

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
