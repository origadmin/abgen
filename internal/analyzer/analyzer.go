package analyzer

import (
	"fmt"
	"go/types"
	"log/slog"
	"os"
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
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return &TypeAnalyzer{
		typeCache: make(map[types.Type]*model.TypeInfo),
	}
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
			slog.Debug("find: obj found", "obj.Name", obj.Name(), "obj.Type", obj.Type().String()) // Added log
			return a.resolveType(obj.Type()), nil
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

	slog.Debug("resolveType", "typ", typ.String(), "type_kind", fmt.Sprintf("%T", typ)) // Added log

	switch t := typ.(type) {
	case *types.Named:
		obj := t.Obj()
		
		// Always set Name, ImportPath, Original, IsAlias from the Named type's object.
		info.Name = obj.Name()
		if obj.Pkg() != nil {
			info.ImportPath = obj.Pkg().Path()
		}
		info.Original = obj
		info.IsAlias = obj.IsAlias()

		// The underlying type is what defines its Kind, Fields, etc.
		// We need to resolve the underlying type first.
		underlyingInfo := a.resolveType(t.Underlying())
		
		// Copy structural information from the underlying type.
		// This ensures aliases and defined types correctly reflect their base structure.
		info.Kind = underlyingInfo.Kind
		info.Fields = underlyingInfo.Fields
		info.Underlying = underlyingInfo
		info.KeyType = underlyingInfo.KeyType
		info.ArrayLen = underlyingInfo.ArrayLen

	case *types.Pointer:
		info.Kind = model.Pointer
		info.Underlying = a.resolveType(t.Elem())
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

func (a *TypeAnalyzer) parseFields(s *types.Struct) []*model.FieldInfo {
	fields := make([]*model.FieldInfo, 0, s.NumFields())
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if !f.Exported() {
			continue
		}

		// If the field is embedded, we recursively get its fields and add them.
		if f.Embedded() {
			embeddedTypeInfo := a.resolveType(f.Type())
			if embeddedTypeInfo != nil && embeddedTypeInfo.Kind == model.Struct {
				fields = append(fields, embeddedTypeInfo.Fields...)
			}
		} else {
			fieldInfo := &model.FieldInfo{
				Name:       f.Name(),
				Type:       a.resolveType(f.Type()),
				Tag:        s.Tag(i),
				IsEmbedded: false, // Explicitly false for promoted fields, as they are no longer "embedded" in this context
			}
			fields = append(fields, fieldInfo)
		}
	}
	return fields
}
