package ast

import (
	"fmt"
	goast "go/ast"
	"go/token"
	gotypes "go/types"
	"log/slog"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/analyzer" // Import analyzer package
	"github.com/origadmin/abgen/internal/types"
)

// DirectiveParser walks the AST of a package, collects directives, and builds a configuration.
type DirectiveParser struct {
	config             *types.ConversionConfig
	userPathAliases    map[string]string // Maps alias from `package:path` directive to full package path
	defaultPathAliases map[string]string // Maps built-in shorthand aliases to full package paths
	currentPkg         *packages.Package
	TypeCache          map[string]*types.TypeInfo // Use types.TypeInfo
	loadedPkgs         map[string]*packages.Package
	packageMode        packages.LoadMode
	allKnownPkgs       []*packages.Package
	localTypeNameToFQN map[string]string // Maps local alias name to its FQN (e.g., "Resource" -> "github.com/origadmin/abgen/testdata/fixture/ent.Resource")
	analyzerWalker     *analyzer.PackageWalker // Analyzer for type resolution
}

// NewDirectiveParser creates a new DirectiveParser.
func NewDirectiveParser() *DirectiveParser {
	return &DirectiveParser{
		config:             types.NewDefaultConfig(),
		userPathAliases:    make(map[string]string),
		defaultPathAliases: map[string]string{
			"time": "time",
			"uuid": "github.com/google/uuid",
			// Add other common packages here
		},
		TypeCache:          make(map[string]*types.TypeInfo), // Use types.TypeInfo
		loadedPkgs:         make(map[string]*packages.Package),
		packageMode:        packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		localTypeNameToFQN: make(map[string]string),
		analyzerWalker:     analyzer.NewPackageWalker(), // Initialize analyzer.PackageWalker
	}
}

// Config returns the final, processed configuration.
func (w *DirectiveParser) Config() *types.ConversionConfig {
	return w.config
}

// AddPackages adds packages to the parser's known packages list.
func (w *DirectiveParser) AddPackages(pkgs ...*packages.Package) {
	if w.allKnownPkgs == nil {
		w.allKnownPkgs = make([]*packages.Package, 0)
	}
	existingPkgs := make(map[string]bool)
	for _, p := range w.allKnownPkgs {
		existingPkgs[p.PkgPath] = true
	}
	for _, newPkg := range pkgs {
		if !existingPkgs[newPkg.PkgPath] {
			w.allKnownPkgs = append(w.allKnownPkgs, newPkg)
			existingPkgs[newPkg.PkgPath] = true
		}
	}
	w.analyzerWalker.AddInitialPackages(pkgs) // Also add to analyzer's packages
}

// Analyze walks the AST of the given package and populates the configuration.
func (w *DirectiveParser) Analyze(pkg *packages.Package) error {
	slog.Info("Analyzing package", "path", pkg.PkgPath)
	w.currentPkg = pkg
	w.config.ContextPackagePath = pkg.PkgPath
	w.AddPackages(pkg)

	for _, file := range pkg.Syntax {
		filename := pkg.Fset.File(file.Pos()).Name()
		if strings.HasSuffix(filepath.Base(filename), ".gen.go") {
			slog.Debug("Skipping generated file", "file", filename)
			continue
		}
		w.collectLocalTypeAliases(file)
		w.processFileDirectives(file)
	}

	if err := w.processPackagePairs(); err != nil {
		return fmt.Errorf("error processing package pairs: %w", err)
	}

	return nil
}

// collectLocalTypeAliases scans the file for `type T = some.OtherType` declarations.
func (w *DirectiveParser) collectLocalTypeAliases(file *goast.File) {
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*goast.TypeSpec); ok {
					if typeSpec.Assign.IsValid() {
						aliasName := typeSpec.Name.Name
						// This part still uses exprToString, which is AST-based.
						// For local aliases, we might still need this, but the FQN should be resolved by analyzer.
						// For now, keep it as is, but note for future refactoring.
						fqn := w.exprToString(typeSpec.Type, w.currentPkg, false)
						w.localTypeNameToFQN[aliasName] = fqn
						slog.Debug("Collected local type alias", "alias", aliasName, "fqn", fqn)
					}
				}
			}
		}
	}
}

// processFileDirectives scans all comment groups in a file and applies any found directives.
func (w *DirectiveParser) processFileDirectives(file *goast.File) {
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			w.parseAndApplyDirective(comment.Text)
		}
	}
}

// parseAndApplyDirective parses a single directive line and applies it to the parser's configuration.
func (w *DirectiveParser) parseAndApplyDirective(line string) {
	key, value := parseDirective(line)
	if key == "" {
		return
	}

	slog.Debug("Processing directive", "key", key, "value", value)
	keys := strings.Split(key, ":")
	verb := keys[0]

	switch verb {
	case "package":
		if len(keys) > 1 && keys[1] == "path" {
			parts := strings.Split(value, ",")
			if len(parts) == 2 && strings.HasPrefix(strings.TrimSpace(parts[1]), "alias=") {
				path := strings.TrimSpace(parts[0])
				alias := strings.TrimPrefix(strings.TrimSpace(parts[1]), "alias=")
				w.userPathAliases[alias] = path
				slog.Debug("Registered user package alias", "alias", alias, "path", path)
			}
		}
	case "pair":
		if len(keys) > 1 && keys[1] == "packages" {
			paths := strings.Split(value, ",")
			if len(paths) == 2 {
				w.config.Source.Package = strings.TrimSpace(paths[0])
				w.config.Target.Package = strings.TrimSpace(paths[1])
				slog.Debug("Set package pair", "source", w.config.Source.Package, "target", w.config.Target.Package)
			}
		}
	case "convert":
		w.applyConversionDirective(keys, value)
	}
}

// applyConversionDirective handles all directives starting with "convert:".
func (w *DirectiveParser) applyConversionDirective(keys []string, value string) {
	if len(keys) == 1 { // convert="A,B"
		parts := strings.Split(value, ",")
		if len(parts) == 2 {
			sourceName := strings.TrimSpace(parts[0])
			targetName := strings.TrimSpace(parts[1])

			sourceInfo, _ := w.Resolve(sourceName)
			targetInfo, _ := w.Resolve(targetName)

			pair := &types.TypePair{
				Source: &types.TypeEndpoint{},
				Target: &types.TypeEndpoint{},
			}
			if sourceInfo != nil {
				pair.Source.Type = sourceInfo.ImportPath + "." + sourceInfo.Name
				pair.Source.AliasType = sourceInfo.LocalAlias
			} else {
				pair.Source.Type = sourceName // Keep as is if unresolved
			}
			if targetInfo != nil {
				pair.Target.Type = targetInfo.ImportPath + "." + targetInfo.Name
				pair.Target.AliasType = targetInfo.LocalAlias
			} else {
				pair.Target.Type = targetName // Keep as is if unresolved
			}
			w.config.Pairs = append(w.config.Pairs, pair)
			slog.Debug("Added type pair", "source", pair.Source.Type, "target", pair.Target.Type)
		}
		return
	}

	subject := keys[1]
	switch subject {
	case "direction":
		w.config.Direction = value
	case "ignore":
		w.config.IgnoreFields[value] = true
	case "remap":
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 {
			w.config.RemapFields[parts[0]] = parts[1]
		}
	case "rule":
		rule := parseCustomRule(value)
		w.config.CustomRules = append(w.config.CustomRules, rule)
	case "source", "target":
		if len(keys) == 3 {
			property := keys[2]
			if subject == "source" {
				if property == "prefix" {
					w.config.Source.Prefix = value
				} else if property == "suffix" {
					w.config.Source.Suffix = value
				}
			} else { // target
				if property == "prefix" {
					w.config.Target.Prefix = value
				} else if property == "suffix" {
					w.config.Target.Suffix = value
				}
			}
		}
	}
}

// processPackagePairs finds matching types for configured package pairs and adds them to the config.
func (w *DirectiveParser) processPackagePairs() error {
	if w.config.Source.Package == "" || w.config.Target.Package == "" {
		return nil
	}

	// Resolve aliases and permanently update the config
	w.config.Source.Package = w.resolveAlias(w.config.Source.Package)
	w.config.Target.Package = w.resolveAlias(w.config.Target.Package)

	slog.Info("Processing package pair", "source", w.config.Source.Package, "target", w.config.Target.Package)
	sourcePkgPath := w.config.Source.Package
	targetPkgPath := w.config.Target.Package

	sourcePkg, err := w.loadPackage(sourcePkgPath)
	if err != nil {
		return fmt.Errorf("failed to load source package %q: %w", sourcePkgPath, err)
	}
	targetPkg, err := w.loadPackage(targetPkgPath)
	if err != nil {
		return fmt.Errorf("failed to load target package %q: %w", targetPkgPath, err)
	}

	sourceTypes := w.getPackageTypes(sourcePkg)
	targetTypes := w.getPackageTypes(targetPkg)

	for typeName := range sourceTypes {
		if _, exists := targetTypes[typeName]; exists {
			pair := &types.TypePair{
				Source: &types.TypeEndpoint{Type: sourcePkg.PkgPath + "." + typeName},
				Target: &types.TypeEndpoint{Type: targetPkg.PkgPath + "." + typeName},
			}

			// Check if source type has a local alias in directives.go
			sourceFQN := sourcePkg.PkgPath + "." + typeName
			for aliasName, fqn := range w.localTypeNameToFQN {
				if fqn == sourceFQN {
					pair.Source.AliasType = aliasName
					break
				}
			}

			// Check if target type has a local alias in directives.go
			targetFQN := targetPkg.PkgPath + "." + typeName
			for aliasName, fqn := range w.localTypeNameToFQN {
				if fqn == targetFQN {
					pair.Target.AliasType = aliasName
					break
				}
			}

			w.config.Pairs = append(w.config.Pairs, pair)
			slog.Debug("Added paired type", "source", pair.Source.Type, "sourceAlias", pair.Source.AliasType, "target", pair.Target.Type, "targetAlias", pair.Target.AliasType)
		}
	}
	return nil
}

// Resolve resolves a type name to its TypeInfo structure.
func (w *DirectiveParser) Resolve(typeName string) (*types.TypeInfo, error) {
	// Delegate to analyzer.PackageWalker for type resolution
	fqn, isLocalAlias := w.findFQNForLocalName(typeName)
	if !isLocalAlias {
		fqn = typeName
	}

	// Resolve package alias if the path part is an alias.
	pkgPath, typeNameOnly := splitFQN(fqn)
	pkgPath = w.resolveAlias(pkgPath)
	if pkgPath == "" {
		pkgPath = w.currentPkg.PkgPath
	}
	fqn = pkgPath + "." + typeNameOnly

	info, err := w.analyzerWalker.FindTypeByFQN(fqn)
	if err != nil {
		return nil, err
	}

	// If it was a local alias, set the LocalAlias field
	if isLocalAlias {
		info.LocalAlias = typeName
	}

	// Cache the resolved type
	w.TypeCache[typeName] = info
	return info, nil
}

// GetTypeCache returns the cache of known types.
func (w *DirectiveParser) GetTypeCache() map[string]*types.TypeInfo {
	return w.TypeCache
}

// GetLocalTypeNameToFQN returns the map of local type aliases to their FQN.
func (w *DirectiveParser) GetLocalTypeNameToFQN() map[string]string {
	return w.localTypeNameToFQN
}

// --- Helper Methods ---

// buildTypeInfo is no longer needed as type resolution is delegated to analyzer.PackageWalker.
// func (w *DirectiveParser) buildTypeInfo(spec *goast.TypeSpec, pkg *packages.Package) *types.TypeInfo {
// 	// This method should be removed or significantly refactored if AST parser needs to build partial TypeInfo.
// 	// For now, it's assumed that full TypeInfo comes from analyzer.
// 	return &types.TypeInfo{}
// }

func (w *DirectiveParser) findPackage(pkgPath string) *packages.Package {
	for _, p := range w.allKnownPkgs {
		if p.PkgPath == pkgPath {
			return p
		}
	}
	return nil
}

func (w *DirectiveParser) findFQNForLocalName(name string) (string, bool) {
	fqn, ok := w.localTypeNameToFQN[name]
	return fqn, ok
}

func (w *DirectiveParser) getPackageTypes(pkg *packages.Package) map[string]bool {
	typesMap := make(map[string]bool)
	if pkg == nil || pkg.Types == nil {
		return typesMap
	}
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		if obj := scope.Lookup(name); obj != nil && obj.Exported() {
			if _, ok := obj.(*gotypes.TypeName); ok {
				typesMap[name] = true
			}
		}
	}
	return typesMap
}

func (w *DirectiveParser) loadPackage(importPath string) (*packages.Package, error) {
	if pkg, ok := w.loadedPkgs[importPath]; ok {
		return pkg, nil
	}
	cfg := &packages.Config{Mode: w.packageMode}
	pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load package %q: %w", importPath, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no package found for import path %q", importPath)
	}
	if packages.PrintErrors(pkgs) > 0 {
		slog.Warn("Errors while loading package", "path", importPath)
	}
	pkg := pkgs[0]
	w.loadedPkgs[pkg.PkgPath] = pkg
	w.AddPackages(pkg) // Add to both ast parser and analyzer walker
	return pkg, nil
}

func (w *DirectiveParser) resolveAlias(aliasOrPath string) string {
	// User-defined aliases take precedence
	if path, ok := w.userPathAliases[aliasOrPath]; ok {
		return path
	}
	// Fallback to default aliases
	if path, ok := w.defaultPathAliases[aliasOrPath]; ok {
		return path
	}
	return aliasOrPath
}

// exprToString resolves an expression to its fully qualified type name, optionally including pointer/slice info.
// This method is still used for parsing local type aliases and should return the FQN.
func (w *DirectiveParser) exprToString(expr goast.Expr, pkg *packages.Package, includeModifiers bool) string {
	if pkg == nil || pkg.TypesInfo == nil {
		return "unknown"
	}

	var typeStr string
	switch e := expr.(type) {
	case *goast.StarExpr: // Pointer type: *T
		if includeModifiers {
			typeStr = "*" + w.exprToString(e.X, pkg, includeModifiers)
		} else {
			typeStr = w.exprToString(e.X, pkg, includeModifiers)
		}
	case *goast.ArrayType: // Slice type: []T
		if includeModifiers {
			typeStr = "[]" + w.exprToString(e.Elt, pkg, includeModifiers)
		} else {
			typeStr = w.exprToString(e.Elt, pkg, includeModifiers)
		}
	case *goast.MapType: // Map type: map[K]V
		if includeModifiers {
			typeStr = "map[" + w.exprToString(e.Key, pkg, includeModifiers) + "]" + w.exprToString(e.Value, pkg, includeModifiers)
		} else {
			typeStr = w.exprToString(e.Value, pkg, includeModifiers) // For simplicity, just return value type for now
		}
	case *goast.Ident: // Identifier: T
		obj := pkg.TypesInfo.ObjectOf(e)
		if obj != nil && obj.Pkg() != nil {
			typeStr = obj.Pkg().Path() + "." + obj.Name()
		} else {
			typeStr = e.Name // Built-in or unresolved
		}
	case *goast.SelectorExpr: // Selector: pkg.T
		obj := pkg.TypesInfo.ObjectOf(e.Sel)
		if obj != nil && obj.Pkg() != nil {
			typeStr = obj.Pkg().Path() + "." + obj.Name()
		} else {
			typeStr = e.Sel.Name // Fallback
		}
	default:
		tv := pkg.TypesInfo.TypeOf(expr)
		if tv != nil {
			typeStr = tv.String()
		} else {
			typeStr = "unknown"
		}
	}

	// Clean up typeStr if it contains package aliases from go/types.TypeString
	// e.g., "github.com/origadmin/abgen/testdata/fixture/ent".User -> github.com/origadmin/abgen/testdata/fixture/ent.User
	if strings.Contains(typeStr, "\"") {
		typeStr = strings.ReplaceAll(typeStr, "\"", "")
		typeStr = strings.ReplaceAll(typeStr, " ", "")
	}

	return typeStr
}

func parseDirective(line string) (key, value string) {
	if !strings.HasPrefix(line, "//go:abgen:") {
		return "", ""
	}
	directive := strings.TrimPrefix(line, "//go:abgen:")
	parts := strings.SplitN(directive, "=", 2)
	key = parts[0]
	if len(parts) > 1 {
		value = strings.Trim(strings.TrimSpace(parts[1]), `"`)
	}
	return key, value
}

func parseCustomRule(value string) types.CustomRule {
	var rule types.CustomRule
	parts := strings.Split(value, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
			switch k {
			case "source":
				rule.SourceTypeName = v
			case "target":
				rule.TargetTypeName = v
			case "func":
				rule.ConvertFunc = v
			}
		}
	}
	return rule
}

func splitFQN(fqn string) (pkgPath, typeName string) {
	lastDot := strings.LastIndex(fqn, ".")
	if lastDot == -1 {
		return "", fqn
	}
	return fqn[:lastDot], fqn[lastDot+1:]
}

func isPointer(expr goast.Expr) bool {
	_, ok := expr.(*goast.StarExpr)
	return ok
}
