package analyzer

import (
	"fmt"
	"go/types"
	"log/slog"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// TypeAnalyzer is responsible for walking through a pre-loaded set of Go packages,
// resolving types, and building a collection of TypeInfo structures.
type TypeAnalyzer struct {
	pkgs      []*packages.Package
	typeCache map[types.Type]*model.TypeInfo
}

// NewTypeAnalyzer creates a new TypeAnalyzer.
func NewTypeAnalyzer() *TypeAnalyzer {
	return &TypeAnalyzer{
		typeCache: make(map[types.Type]*model.TypeInfo),
	}
}

// SetPackages sets the packages for testing purposes
func (a *TypeAnalyzer) SetPackages(pkgs []*packages.Package) {
	a.pkgs = pkgs
}

// Find is a public wrapper for the find method
func (a *TypeAnalyzer) Find(fqn string) (*model.TypeInfo, error) {
	return a.find(fqn)
}

// Analyze is the main entry point for the analysis phase.
func (a *TypeAnalyzer) Analyze(cfg *config.Config) (map[string]*model.TypeInfo, error) {
	seedPaths := a.collectSeedPaths(cfg)

	loadCfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Tests:      false,
		BuildFlags: []string{"-tags=abgen"},
	}

	pkgs, err := packages.Load(loadCfg, seedPaths...)
	if err != nil {
		return nil, fmt.Errorf("failed to load package graph: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("packages contain errors")
	}
	a.pkgs = pkgs

	resolvedTypes := make(map[string]*model.TypeInfo)
	for _, rule := range cfg.ConversionRules {
		if _, ok := resolvedTypes[rule.SourceType]; !ok {
			info, err := a.find(rule.SourceType)
			if err != nil {
				slog.Warn("could not resolve source type", "fqn", rule.SourceType, "error", err)
				continue
			}
			resolvedTypes[rule.SourceType] = info
		}
		if _, ok := resolvedTypes[rule.TargetType]; !ok {
			info, err := a.find(rule.TargetType)
			if err != nil {
				slog.Warn("could not resolve target type", "fqn", rule.TargetType, "error", err)
				continue
			}
			resolvedTypes[rule.TargetType] = info
		}
	}

	return resolvedTypes, nil
}

func (a *TypeAnalyzer) collectSeedPaths(cfg *config.Config) []string {
	pathMap := make(map[string]struct{})
	if cfg.GenerationContext.PackagePath != "" {
		pathMap[cfg.GenerationContext.PackagePath] = struct{}{}
	}
	for _, pair := range cfg.PackagePairs {
		pathMap[pair.SourcePath] = struct{}{}
		pathMap[pair.TargetPath] = struct{}{}
	}
	for _, rule := range cfg.ConversionRules {
		if pkgPath := getPkgPath(rule.SourceType); pkgPath != "" {
			pathMap[pkgPath] = struct{}{}
		}
		if pkgPath := getPkgPath(rule.TargetType); pkgPath != "" {
			pathMap[pkgPath] = struct{}{}
		}
	}
	paths := make([]string, 0, len(pathMap))
	for path := range pathMap {
		paths = append(paths, path)
	}
	return paths
}

func getPkgPath(fqn string) string {
	lastDot := strings.LastIndex(fqn, ".")
	if lastDot == -1 {
		return ""
	}
	return fqn[:lastDot]
}

func (a *TypeAnalyzer) find(fqn string) (*model.TypeInfo, error) {
	pkgPath := getPkgPath(fqn)
	if pkgPath == "" {
		return nil, fmt.Errorf("invalid FQN, must include package path: %s", fqn)
	}
	typeName := fqn[len(pkgPath)+1:]

	slog.Debug("find", "fqn", fqn, "pkgPath", pkgPath, "typeName", typeName) // Added log

	for _, pkg := range a.pkgs {
		if pkg.PkgPath == pkgPath {
			slog.Debug("find: found matching pkg", "pkg.PkgPath", pkg.PkgPath) // Added log
			scope := pkg.Types.Scope()
			if scope == nil {
				slog.Debug("find: scope is nil", "pkg.PkgPath", pkg.PkgPath) // Added log
				continue
			}
			obj := scope.Lookup(typeName)
			if obj == nil {
				slog.Debug("find: obj not found in scope", "typeName", typeName, "pkg.PkgPath", pkg.PkgPath) // Added log
				continue
			}
			objType := obj.Type()
			slog.Debug("find: obj found", "obj.Name", obj.Name(), "obj.Type", objType.String(), "obj.TypeKind", fmt.Sprintf("%T", objType)) // Added log
			return a.resolveType(objType), nil
		}
	}
	slog.Debug("find: type not found in any loaded pkg", "fqn", fqn) // Added log
	return nil, fmt.Errorf("type %q not found in pre-loaded packages", fqn)
}

func (a *TypeAnalyzer) resolveType(typ types.Type) *model.TypeInfo {
	if typ == nil {
		return nil
	}
	if cached, exists := a.typeCache[typ]; exists {
		return cached
	}

	info := &model.TypeInfo{}
	a.typeCache[typ] = info

	typeKind := fmt.Sprintf("%T", typ)
	slog.Debug("resolveType", "typ", typ.String(), "type_kind", typeKind) // Added log

	switch t := typ.(type) {
	case *types.Alias:
		obj := t.Obj()
		info.Name = obj.Name()
		if obj.Pkg() != nil {
			info.ImportPath = obj.Pkg().Path()
		}
		info.Original = obj
		info.IsAlias = true

		slog.Debug("resolveType: Alias", "name", info.Name, "importPath", info.ImportPath, "rhs", t.Rhs().String())

		// For a type alias (type T = U), we use Rhs() to get the aliased type
		// Rhs() returns the type on the right-hand side of the alias declaration
		underlyingInfo := a.resolveType(t.Rhs())
		
		if underlyingInfo != nil {
			info.Kind = model.Named
			info.Underlying = underlyingInfo
		}

	case *types.Named:
		obj := t.Obj()

		info.Name = obj.Name()
		if obj.Pkg() != nil {
			info.ImportPath = obj.Pkg().Path()
		}
		info.Original = obj
		info.IsAlias = obj.IsAlias()

		slog.Debug("resolveType: Named", "name", info.Name, "importPath", info.ImportPath, "isAlias", info.IsAlias, "underlying", t.Underlying().String())

		underlyingInfo := a.resolveType(t.Underlying())
		if underlyingInfo != nil {
			// If the underlying type is a struct, then this named type IS a struct.
			// Its Kind should be Struct, and it should expose the fields directly.
			if underlyingInfo.Kind == model.Struct {
				info.Kind = model.Struct
				info.Fields = underlyingInfo.Fields
			} else {
				// Otherwise, this is a defined type acting as an alias for a non-struct type.
				// Its Kind is Named, and it points to the underlying type info.
				info.Kind = model.Named
				info.Underlying = underlyingInfo
			}
		}

	case *types.Pointer:
		info.Kind = model.Pointer
		underlyingInfo := a.resolveType(t.Elem())
		info.Underlying = underlyingInfo

		slog.Debug("resolveType: Pointer", "elem", t.Elem().String(), "underlyingInfo.Name", func() string {
			if underlyingInfo != nil {
				return underlyingInfo.Name
			} else {
				return "nil"
			}
		}())

		// If the pointer element is a named type, we can infer the pointer's name
		if underlyingInfo != nil && underlyingInfo.Name != "" {
			// For pointers, we don't set a name since they are anonymous types
			// The name should be represented by the underlying type
		}
	case *types.Slice:
		info.Kind = model.Slice
		info.Underlying = a.resolveType(t.Elem())
	case *types.Array:
		info.Kind = model.Array
		info.ArrayLen = int(t.Len())
		info.Underlying = a.resolveType(t.Elem())
	case *types.Map:
		info.Kind = model.Map
		info.KeyType = a.resolveType(t.Key())
		info.Underlying = a.resolveType(t.Elem())
	case *types.Struct:
		info.Kind = model.Struct
		info.Fields = a.parseFields(t)
	case *types.Basic:
		info.Kind = model.Primitive
		info.Name = t.Name()
	case *types.Interface:
		info.Kind = model.Interface
		info.Name = "interface{}"
	default:
		info.Kind = model.Unknown
		info.Name = t.String()
	}
	return info
}

// findOriginalNamedType tries to find the original named type that matches the given type structure
func (a *TypeAnalyzer) findOriginalNamedType(typ types.Type) *model.TypeInfo {
	if structType, ok := typ.(*types.Struct); ok {
		// Search through all loaded packages to find a matching named struct type
		for _, pkg := range a.pkgs {
			scope := pkg.Types.Scope()
			if scope == nil {
				continue
			}

			for _, name := range scope.Names() {
				obj := scope.Lookup(name)
				if obj == nil {
					continue
				}

				// Check if this is a named type with struct underlying
				if namedType, ok := obj.Type().(*types.Named); ok {
					if namedUnderlying, ok := namedType.Underlying().(*types.Struct); ok {
						// Compare the struct signatures
						if a.structsAreEqual(structType, namedUnderlying) {
							slog.Debug("findOriginalNamedType: found match", "name", obj.Name(), "pkg", obj.Pkg().Path())
							return a.resolveType(obj.Type())
						}
					}
				}
			}
		}
	}
	return nil
}

// structsAreEqual compares two structs for field equality
func (a *TypeAnalyzer) structsAreEqual(s1, s2 *types.Struct) bool {
	if s1.NumFields() != s2.NumFields() {
		return false
	}

	for i := 0; i < s1.NumFields(); i++ {
		f1 := s1.Field(i)
		f2 := s2.Field(i)

		if f1.Name() != f2.Name() {
			return false
		}

		if !types.Identical(f1.Type(), f2.Type()) {
			return false
		}
	}

	return true
}

// resolveNamedType resolves a named type by its object, preserving its identity
func (a *TypeAnalyzer) resolveNamedType(obj *types.TypeName) *model.TypeInfo {
	if obj == nil {
		return nil
	}

	// Create TypeInfo with the named type's identity
	info := &model.TypeInfo{
		Name:     obj.Name(),
		IsAlias:  obj.IsAlias(),
		Original: obj,
	}

	if obj.Pkg() != nil {
		info.ImportPath = obj.Pkg().Path()
	}

	// Now resolve the underlying structure
	underlyingType := obj.Type().Underlying()
	underlyingInfo := a.resolveType(underlyingType)
	if underlyingInfo != nil {
		info.Kind = underlyingInfo.Kind
		info.Fields = underlyingInfo.Fields
		info.KeyType = underlyingInfo.KeyType
		info.ArrayLen = underlyingInfo.ArrayLen
		info.Underlying = underlyingInfo.Underlying
	}

	return info
}

func (a *TypeAnalyzer) parseFields(s *types.Struct) []*model.FieldInfo {
	fields := make([]*model.FieldInfo, 0, s.NumFields())
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)

		// Only process exported fields for direct inclusion.
		// Embedded fields need special handling to "promote" their exported fields.
		if f.Embedded() {
			embeddedTypeInfo := a.resolveType(f.Type())
			if embeddedTypeInfo != nil && embeddedTypeInfo.Kind == model.Struct {
				// Promote exported fields from the embedded struct
				for _, embeddedField := range embeddedTypeInfo.Fields {
					// Only add if the embedded field itself is exported (already filtered in embeddedTypeInfo.Fields)
					// and if it's not an embedded field of the embedded struct (avoid double promotion if not desired)
					// For now, just add all fields from the embedded struct's Fields slice, as parseFields already filters for exported.
					fields = append(fields, embeddedField)
				}
			}
			continue // The embedded field itself is not added as a direct field, its contents are promoted.
		}

		if !f.Exported() {
			continue
		}

		fieldInfo := &model.FieldInfo{
			Name:       f.Name(),
			Type:       a.resolveType(f.Type()),
			Tag:        s.Tag(i),
			IsEmbedded: false, // Direct fields are not embedded
		}
		fields = append(fields, fieldInfo)
	}
	return fields
}
