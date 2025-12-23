package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log/slog"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/generator/components"
	"github.com/origadmin/abgen/internal/model"
	"github.com/origadmin/abgen/internal/planner"
)

// TypeAnalyzer is responsible for parsing Go source files, extracting directives,
// configuring the build, and analyzing all required types.
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

// Analyze is the main entry point for the analysis phase. It orchestrates
// the loading of the initial package, parsing of directives, and deep analysis of all related types.
func (a *TypeAnalyzer) Analyze(sourceDir string) (*model.AnalysisResult, error) {
	// 1. Load the initial package to find directives and get context.
	initialPkg, err := a.loadInitialPackage(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load initial package: %w", err)
	}
	if initialPkg == nil {
		return nil, fmt.Errorf("no initial package found at %s", sourceDir)
	}

	// 2. Extract directives from the initial package's syntax files.
	directives := a.extractDirectives(initialPkg)

	// 3. Parse the extracted directives to build the initial configuration.
	cfgParser := config.NewParser()
	initialConfig, err := cfgParser.ParseDirectives(directives, initialPkg.Name, initialPkg.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directives: %w", err)
	}
	initialConfig.GenerationContext.DirectivePath = sourceDir

	// 4. Analyze all required external packages based on the configuration.
	resolvedTypes, err := a.analyzeExternalPackages(initialConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze external packages: %w", err)
	}

	// 5. Discover existing definitions (aliases, functions) in the initial package.
	existingFuncs, existingAliases := a.discoverExistingDefinitions(initialPkg)

	// 6. Create the final execution plan.
	typeConverter := components.NewTypeConverter()
	planner := planner.NewPlanner(typeConverter)
	executionPlan := planner.Plan(initialConfig, resolvedTypes)

	// 7. Assemble the final analysis result.
	analysisResult := &model.AnalysisResult{
		TypeInfos:         resolvedTypes,
		ExistingFunctions: existingFuncs,
		ExistingAliases:   existingAliases,
		ExecutionPlan:     executionPlan,
	}

	return analysisResult, nil
}

// loadInitialPackage loads the package at the given source directory.
func (a *TypeAnalyzer) loadInitialPackage(sourceDir string) (*packages.Package, error) {
	initialLoaderCfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedSyntax | packages.NeedFiles | packages.NeedModule |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir:        sourceDir,
		Tests:      false,
		BuildFlags: []string{"-tags=abgen_source"},
	}

	initialPkgs, err := packages.Load(initialLoaderCfg, ".")
	if err != nil {
		return nil, fmt.Errorf("error during package loading at %s: %w", sourceDir, err)
	}
	for _, pkg := range initialPkgs {
		for _, pkgErr := range pkg.Errors {
			slog.Warn("Initial package contains errors, analysis may be incomplete",
				"pkg", pkg.PkgPath, "error", pkgErr.Error(), "pos", pkgErr.Pos)
		}
	}
	if len(initialPkgs) == 0 {
		return nil, nil // No error, but no package.
	}
	return initialPkgs[0], nil
}

// extractDirectives scans all files in a package for abgen directives.
func (a *TypeAnalyzer) extractDirectives(pkg *packages.Package) []string {
	var directives []string
	if pkg == nil {
		return directives
	}
	for _, file := range pkg.Syntax {
		for _, commentGroup := range file.Comments {
			for _, comment := range commentGroup.List {
				if strings.HasPrefix(comment.Text, "//go:abgen:") {
					directives = append(directives, strings.TrimSpace(comment.Text))
				}
			}
		}
	}
	return directives
}

// analyzeExternalPackages loads and performs a deep type analysis on the specified external packages.
func (a *TypeAnalyzer) analyzeExternalPackages(cfg *config.Config) (map[string]*model.TypeInfo, error) {
	paths := a.collectExternalPaths(cfg)
	if len(paths) == 0 {
		return make(map[string]*model.TypeInfo), nil
	}

	loadCfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedModule |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Tests: false,
	}
	pkgs, err := packages.Load(loadCfg, paths...)
	if err != nil {
		return nil, fmt.Errorf("failed to load external package graph: %w", err)
	}
	a.pkgs = pkgs

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("error loading external package %s: %v", pkg.PkgPath, pkg.Errors)
		}
	}

	resolvedTypes := make(map[string]*model.TypeInfo)

	for _, pkg := range a.pkgs {
		for _, name := range pkg.Types.Scope().Names() {
			obj := pkg.Types.Scope().Lookup(name)
			if obj != nil {
				if _, ok := obj.(*types.TypeName); ok {
					if named, ok := obj.Type().(*types.Named); ok {
						if _, isStruct := named.Underlying().(*types.Struct); isStruct {
							fqn := named.Obj().Pkg().Path() + "." + named.Obj().Name()
							if _, ok := resolvedTypes[fqn]; !ok {
								info := a.resolveType(named)
								if info != nil {
									resolvedTypes[fqn] = info
								}
							}
						}
					}
				}
			}
		}
	}

	for _, fqn := range a.collectAllFqns(cfg) {
		if _, ok := resolvedTypes[fqn]; !ok {
			info, err := a.find(fqn)
			if err != nil {
				slog.Warn("could not resolve explicitly required type", "fqn", fqn, "error", err)
				continue
			}
			resolvedTypes[fqn] = info
		}
	}

	return resolvedTypes, nil
}

// discoverExistingDefinitions performs a lightweight, AST-based analysis of the local package.
func (a *TypeAnalyzer) discoverExistingDefinitions(pkg *packages.Package) (map[string]bool, map[string]string) {
	existingFunctions := make(map[string]bool)
	existingAliases := make(map[string]string)

	if pkg == nil {
		return existingFunctions, existingAliases
	}

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name != nil {
					existingFunctions[d.Name.Name] = true
				}
			case *ast.GenDecl:
				if d.Tok == token.TYPE {
					for _, spec := range d.Specs {
						if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name != nil && ts.Assign > 0 {
							fqn := a.resolveAliasTargetFQN(ts.Type, pkg)
							existingAliases[ts.Name.Name] = fqn
						}
					}
				}
			}
		}
	}

	return existingFunctions, existingAliases
}

// resolveAliasTargetFQN converts an alias's target type expression to a fully qualified name.
func (a *TypeAnalyzer) resolveAliasTargetFQN(expr ast.Expr, pkg *packages.Package) string {
	typeStr := types.ExprString(expr)

	if selExpr, ok := expr.(*ast.SelectorExpr); ok {
		if pkgIdent, ok := selExpr.X.(*ast.Ident); ok {
			pkgAlias := pkgIdent.Name
			typeName := selExpr.Sel.Name

			for importPath, importedPkg := range pkg.Imports {
				if importedPkg.Name == pkgAlias {
					return importPath + "." + typeName
				}
			}
		}
	}

	if !strings.Contains(typeStr, ".") {
		return pkg.ID + "." + typeStr
	}

	return typeStr
}

// collectExternalPaths gathers all package paths that need full type analysis.
func (a *TypeAnalyzer) collectExternalPaths(cfg *config.Config) []string {
	pathMap := make(map[string]struct{})

	for _, path := range cfg.PackageAliases {
		pathMap[path] = struct{}{}
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
	for key := range cfg.CustomFunctionRules {
		parts := strings.Split(key, "->")
		if len(parts) == 2 {
			if pkgPath := getPkgPath(parts[0]); pkgPath != "" {
				pathMap[pkgPath] = struct{}{}
			}
			if pkgPath := getPkgPath(parts[1]); pkgPath != "" {
				pathMap[pkgPath] = struct{}{}
			}
		}
	}

	paths := make([]string, 0, len(pathMap))
	for path := range pathMap {
		paths = append(paths, path)
	}
	return paths
}

// collectAllFqns gathers all unique fully-qualified type names (FQNs).
func (a *TypeAnalyzer) collectAllFqns(cfg *config.Config) []string {
	fqnMap := make(map[string]struct{})

	for _, rule := range cfg.ConversionRules {
		if rule.SourceType != "" {
			fqnMap[rule.SourceType] = struct{}{}
		}
		if rule.TargetType != "" {
			fqnMap[rule.TargetType] = struct{}{}
		}
	}
	for key := range cfg.CustomFunctionRules {
		parts := strings.Split(key, "->")
		if len(parts) == 2 {
			if parts[0] != "" {
				fqnMap[parts[0]] = struct{}{}
			}
			if parts[1] != "" {
				fqnMap[parts[1]] = struct{}{}
			}
		}
	}

	fqns := make([]string, 0, len(fqnMap))
	for fqn := range fqnMap {
		fqns = append(fqns, fqn)
	}
	return fqns
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

	for _, pkg := range a.pkgs {
		if pkg.PkgPath == pkgPath {
			scope := pkg.Types.Scope()
			if scope == nil {
				continue
			}
			obj := scope.Lookup(typeName)
			if obj == nil {
				continue
			}
			return a.resolveType(obj.Type()), nil
		}
	}
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

	switch t := typ.(type) {
	case *types.Alias:
		obj := t.Obj()
		info.Name = obj.Name()
		if obj.Pkg() != nil {
			info.ImportPath = obj.Pkg().Path()
		}
		info.Original = obj
		info.IsAlias = true
		underlyingInfo := a.resolveType(t.Rhs())
		info.Kind = model.Named
		info.Underlying = underlyingInfo

	case *types.Named:
		obj := t.Obj()
		info.Name = obj.Name()
		if obj.Pkg() != nil {
			info.ImportPath = obj.Pkg().Path()
		}
		info.Original = obj
		underlyingInfo := a.resolveType(t.Underlying())
		if underlyingInfo != nil {
			if underlyingInfo.Kind == model.Struct {
				info.Kind = model.Struct
				info.Fields = underlyingInfo.Fields
			} else {
				info.Kind = model.Named
				info.Underlying = underlyingInfo
			}
		}

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

		if f.Embedded() {
			embeddedTypeInfo := a.resolveType(f.Type())
			if embeddedTypeInfo != nil && (embeddedTypeInfo.Kind == model.Struct || (embeddedTypeInfo.Kind == model.Named && embeddedTypeInfo.Underlying.Kind == model.Struct)) {
				var embeddedFields []*model.FieldInfo
				if embeddedTypeInfo.Kind == model.Struct {
					embeddedFields = embeddedTypeInfo.Fields
				} else {
					embeddedFields = embeddedTypeInfo.Underlying.Fields
				}

				for _, embeddedField := range embeddedFields {
					fields = append(fields, embeddedField)
				}
			}
			continue
		}

		if !f.Exported() {
			continue
		}

		fieldInfo := &model.FieldInfo{
			Name:       f.Name(),
			Type:       a.resolveType(f.Type()),
			Tag:        s.Tag(i),
			IsEmbedded: false,
		}
		fields = append(fields, fieldInfo)
	}
	return fields
}
