package analyzer

import (
	"fmt"
	"go/types"
	"log/slog"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/types" // Import the new types package
)

const abgenDirectivePrefix = "//go:abgen:"

// PackageWalker is responsible for walking through Go packages,
// resolving types, and building a collection of TypeInfo structures.
type PackageWalker struct {
	pkgs        []*packages.Package
	typeCache   map[string]*types.TypeInfo // Use types.TypeInfo
	failedLoads map[string]bool
}

// NewPackageWalker creates a new PackageWalker.
func NewPackageWalker() *PackageWalker {
	return &PackageWalker{
		typeCache:   make(map[string]*types.TypeInfo), // Use types.TypeInfo
		failedLoads: make(map[string]bool),
	}
}

// FindTypeByFQN is the main entry point for resolving a type.
// It orchestrates package loading and type resolution.
func (w *PackageWalker) FindTypeByFQN(fqn string) (*types.TypeInfo, error) { // Use types.TypeInfo
	// Check cache first
	if info, ok := w.typeCache[fqn]; ok {
		return info, nil
	}

	// Resolve the type, which may involve loading packages
	info, err := w.resolveTypeByFQN(fqn)
	if err != nil {
		return nil, err
	}

	// Cache the successfully resolved type
	w.typeCache[fqn] = info
	return info, nil
}

// resolveTypeByFQN performs the core logic of finding and resolving a type.
func (w *PackageWalker) resolveTypeByFQN(fqn string) (*types.TypeInfo, error) { // Use types.TypeInfo
	pkgPath, typeName := splitFQN(fqn)

	// Handle built-in types
	if pkgPath == "" {
		if isPrimitiveType(typeName) {
			return &types.TypeInfo{Name: typeName, Kind: types.Primitive}, nil // Use types.Primitive
		}
		return nil, fmt.Errorf("cannot resolve built-in or unqualified type: %q", typeName)
	}

	// Find the package, loading it if necessary
	pkg, err := w.findOrLoadPackage(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find or load package %q for type %q: %w", pkgPath, fqn, err)
	}

	// Lookup the type name in the package's scope
	obj := pkg.Types.Scope().Lookup(typeName)
	if obj == nil {
		return nil, fmt.Errorf("type %q not found in package %q", typeName, pkgPath)
	}

	tn, ok := obj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("%q in package %q is not a type name", typeName, pkgPath)
	}

	// Now, resolve the found type object into our TypeInfo struct
	return w.resolveType(tn.Type())
}

// findOrLoadPackage finds a package in the walker's list or loads it dynamically.
func (w *PackageWalker) findOrLoadPackage(pkgPath string) (*packages.Package, error) {
	// Check if already loaded
	for _, pkg := range w.pkgs {
		if pkg.PkgPath == pkgPath {
			return pkg, nil
		}
	}

	// Check if we already tried and failed to load this package
	if w.failedLoads[pkgPath] {
		return nil, fmt.Errorf("package previously failed to load")
	}

	// If not found, load it
	slog.Debug("Dynamically loading missing package", "path", pkgPath)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Tests:      false,
		BuildFlags: []string{"-tags=abgen"},
	}

	loadedPkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		w.failedLoads[pkgPath] = true
		return nil, fmt.Errorf("packages.Load failed: %w", err)
	}
	if packages.PrintErrors(loadedPkgs) > 0 {
		w.failedLoads[pkgPath] = true
		return nil, fmt.Errorf("loaded package contains errors")
	}
	if len(loadedPkgs) == 0 {
		w.failedLoads[pkgPath] = true
		return nil, fmt.Errorf("no package found for import path")
	}

	newPkg := loadedPkgs[0]
	w.pkgs = append(w.pkgs, newPkg)
	slog.Debug("Successfully loaded and added package", "path", newPkg.PkgPath)
	return newPkg, nil
}

// typeUnwindInfo collects information during the type unwinding process.
type typeUnwindInfo struct {
	kind           types.TypeKind // Use types.TypeKind
	arrayLen       int
	isAlias        bool
	original       types.Object
	baseNamedType  *types.TypeName // The innermost named type found
	effectiveType  types.Type      // The final non-alias, non-named, non-composite type (e.g., *types.Struct, *types.Basic)
	underlyingType types.Type      // The direct element type for the current level of modification (e.g., for *T, it's T)
	keyType        types.Type      // For map, the key type
}

// unwindType recursively unwinds aliases and named types to find the effective base type
// and collect all modifiers.
func (w *PackageWalker) unwindType(typ types.Type) typeUnwindInfo {
	info := typeUnwindInfo{}
	currentType := typ

	for {
		switch t := currentType.(type) {
		case *types.Alias:
			slog.Debug("Unwinding *types.Alias", "type", t.String())
			if info.original == nil { // Capture the first original object for the top-level type
				info.original = t.Obj()
				info.isAlias = true
			}
			if info.baseNamedType == nil { // Capture the first named type encountered
				info.baseNamedType = t.Obj()
			}
			currentType = t.Rhs() // Continue unwinding the aliased type

		case *types.Named:
			slog.Debug("Unwinding *types.Named", "type", t.String())
			if info.original == nil { // Capture the first original object for the top-level type
				info.original = t.Obj()
				info.isAlias = t.Obj().IsAlias()
			}
			if info.baseNamedType == nil { // Capture the first named type encountered
				info.baseNamedType = t.Obj()
			}
			currentType = t.Underlying() // Continue unwinding the underlying type

		case *types.Pointer:
			slog.Debug("Unwinding *types.Pointer", "type", t.String())
			info.kind = types.Pointer // Use types.Pointer
			info.underlyingType = t.Elem()
			currentType = t.Elem()

		case *types.Slice:
			slog.Debug("Unwinding *types.Slice", "type", t.String())
			info.kind = types.Slice // Use types.Slice
			info.underlyingType = t.Elem()
			currentType = t.Elem()

		case *types.Array:
			slog.Debug("Unwinding *types.Array", "type", t.String())
			info.kind = types.Array // Use types.Array
			info.arrayLen = int(t.Len())
			info.underlyingType = t.Elem()
			currentType = t.Elem()

		case *types.Map:
			slog.Debug("Unwinding *types.Map", "type", t.String())
			info.kind = types.Map // Use types.Map
			info.keyType = t.Key()
			info.underlyingType = t.Elem() // Value type
			currentType = t.Elem()

		case *types.Basic:
			slog.Debug("Unwinding *types.Basic", "type", t.String())
			info.kind = types.Primitive // Use types.Primitive
			info.effectiveType = t
			return info

		case *types.Struct:
			slog.Debug("Unwinding *types.Struct", "type", t.String())
			info.kind = types.Struct // Use types.Struct
			info.effectiveType = t
			return info

		case *types.Interface:
			slog.Debug("Unwinding *types.Interface", "type", t.String())
			info.kind = types.Interface // Use types.Interface
			info.effectiveType = t
			return info

		case *types.Chan:
			slog.Debug("Unwinding *types.Chan", "type", t.String())
			info.kind = types.Chan // Use types.Chan
			info.effectiveType = t
			return info

		case *types.Signature:
			slog.Debug("Unwinding *types.Signature", "type", t.String())
			info.kind = types.Func // Use types.Func
			info.effectiveType = t
			return info

		default:
			slog.Warn("unhandled type during unwind", "type", t.String(), "typeOf", fmt.Sprintf("%T", t))
			info.kind = types.Unknown // Use types.Unknown
			info.effectiveType = t
			return info
		}

		// If currentType is the same as typ, it means we didn't unwind anything,
		// so baseType should be typ. Otherwise, baseType is the innermost type.
		if currentType == typ {
			break // Avoid infinite loop if type doesn't unwind further
		}
		typ = currentType // Update typ for the next iteration
	}

	info.effectiveType = currentType // The last unwound type is the effective type
	return info
}

// resolveType is the recursive worker for building the TypeInfo graph.
func (w *PackageWalker) resolveType(typ types.Type) *types.TypeInfo { // Use types.TypeInfo
	if typ == nil {
		return nil
	}

	// Unwind the type to get its effective kind and base named type
	unwindInfo := w.unwindType(typ)

	// Determine the cache key. Only cache named types by their FQN.
	var cacheKey string
	if unwindInfo.original != nil { // If the top-level type is a named type or alias
		cacheKey = unwindInfo.original.Pkg().Path() + "." + unwindInfo.original.Name()
		if cached, exists := w.typeCache[cacheKey]; exists {
			return cached
		}
	}

	info := &types.TypeInfo{ // Use types.TypeInfo
		Kind:       unwindInfo.kind,
		ArrayLen:   unwindInfo.arrayLen,
		IsAlias:    unwindInfo.isAlias,
		Original:   unwindInfo.original,
	}

	// Pre-cache for recursion, only for named types to prevent cycles with anonymous types.
	if cacheKey != "" {
		w.typeCache[cacheKey] = info
	}

	// Populate Name and ImportPath for the top-level named type if applicable
	if unwindInfo.original != nil {
		info.Name = unwindInfo.original.Name()
		if unwindInfo.original.Pkg() != nil {
			info.ImportPath = unwindInfo.original.Pkg().Path()
		}
	} else if basic, ok := typ.(*types.Basic); ok { // Handle primitive types directly if not named
		info.Name = basic.Name()
	} else if structType, ok := typ.(*types.Struct); ok { // Handle anonymous structs
		info.Name = "struct{...}"
	} else if interfaceType, ok := typ.(*types.Interface); ok { // Handle anonymous interfaces
		info.Name = "interface{...}"
	}

	// Resolve Underlying and KeyType
	if unwindInfo.underlyingType != nil {
		info.Underlying = w.resolveType(unwindInfo.underlyingType)
		if info.Underlying != nil && info.Underlying.FQN() != "" {
			info.UnderlyingFQN = info.Underlying.FQN()
		}
	}
	if unwindInfo.keyType != nil {
		info.KeyType = w.resolveType(unwindInfo.keyType)
		if info.KeyType != nil && info.KeyType.FQN() != "" {
			info.KeyTypeFQN = info.KeyType.FQN()
		}
	}

	// Handle struct fields
	if info.Kind == types.Struct { // Use types.Struct
		if structType, ok := unwindInfo.effectiveType.(*types.Struct); ok {
			info.Fields = w.parseFields(structType)
		}
	}

	return info
}

// parseFields parses the fields of a *types.Struct.
func (w *PackageWalker) parseFields(s *types.Struct) []*types.FieldInfo { // Use types.FieldInfo
	fields := make([]*types.FieldInfo, 0, s.NumFields()) // Use types.FieldInfo
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if !f.Exported() {
			continue
		}
		fields = append(fields, &types.FieldInfo{ // Use types.FieldInfo
			Name:       f.Name(),
			Type:       w.resolveType(f.Type()),
			Tag:        s.Tag(i),
			IsEmbedded: f.Embedded(),
		})
	}
	return fields
}

// AddInitialPackages seeds the walker with a set of pre-loaded packages.
func (w *PackageWalker) AddInitialPackages(pkgs []*packages.Package) {
	w.pkgs = append(w.pkgs, pkgs...)
}

// --- Helper Functions ---

func splitFQN(fqn string) (pkgPath, typeName string) {
	lastDot := strings.LastIndex(fqn, ".")
	if lastDot == -1 {
		return "", fqn
	}
	return fqn[:lastDot], fqn[lastDot+1:]
}

func isPrimitiveType(name string) bool {
	// A simplified check for Go's built-in primitive types.
	switch name {
	case "bool", "byte", "rune", "string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128", "error":
		return true
	default:
		return false
	}
}
