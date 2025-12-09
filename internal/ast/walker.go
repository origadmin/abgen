// Package ast implements the functions, types, and interfaces for the module.
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

	"github.com/origadmin/abgen/internal/types"
)

// PackageWalker walks the AST of a package, collects directives, and builds a configuration.
type PackageWalker struct {
	config             *types.ConversionConfig
	pathAliases        map[string]string // Maps alias from `package:path` directive to full package path
	currentPkg         *packages.Package
	TypeCache          map[string]*types.TypeInfo
	loadedPkgs         map[string]*packages.Package
	packageMode        packages.LoadMode
	allKnownPkgs       []*packages.Package
	localTypeNameToFQN map[string]string
}

// NewPackageWalker creates a new PackageWalker.
func NewPackageWalker() *PackageWalker {
	return &PackageWalker{
		config:             types.NewDefaultConfig(),
		pathAliases:        make(map[string]string),
		TypeCache:          make(map[string]*types.TypeInfo),
		loadedPkgs:         make(map[string]*packages.Package),
		packageMode:        packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		localTypeNameToFQN: make(map[string]string),
	}
}

// Config returns the final, processed configuration.
func (w *PackageWalker) Config() *types.ConversionConfig {
	return w.config
}

// AddPackages adds packages to the walker's known packages list.
func (w *PackageWalker) AddPackages(pkgs ...*packages.Package) {
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
}

// Analyze walks the AST of the given package and populates the configuration.
func (w *PackageWalker) Analyze(pkg *packages.Package) error {
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
func (w *PackageWalker) collectLocalTypeAliases(file *goast.File) {
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*goast.TypeSpec); ok {
					if typeSpec.Assign.IsValid() {
						aliasName := typeSpec.Name.Name
						fqn := w.exprToString(typeSpec.Type, w.currentPkg)
						w.localTypeNameToFQN[aliasName] = fqn
						slog.Debug("Collected local type alias", "alias", aliasName, "fqn", fqn)
					}
				}
			}
		}
	}
}

// processFileDirectives scans all comment groups in a file and applies any found directives.
func (w *PackageWalker) processFileDirectives(file *goast.File) {
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			w.parseAndApplyDirective(comment.Text)
		}
	}
}

// parseAndApplyDirective parses a single directive line and applies it to the walker's configuration.
func (w *PackageWalker) parseAndApplyDirective(line string) {
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
				w.pathAliases[alias] = path
				slog.Debug("Registered package alias", "alias", alias, "path", path)
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
func (w *PackageWalker) applyConversionDirective(keys []string, value string) {
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
func (w *PackageWalker) processPackagePairs() error {
	if w.config.Source.Package == "" || w.config.Target.Package == "" {
		return nil
	}

	slog.Info("Processing package pair", "source", w.config.Source.Package, "target", w.config.Target.Package)
	sourcePkgPath := w.resolveAlias(w.config.Source.Package)
	targetPkgPath := w.resolveAlias(w.config.Target.Package)

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
			w.config.Pairs = append(w.config.Pairs, pair)
			slog.Debug("Added paired type", "source", pair.Source.Type, "target", pair.Target.Type)
		}
	}
	return nil
}

// Resolve resolves a type name to its TypeInfo structure.
func (w *PackageWalker) Resolve(typeName string) (*types.TypeInfo, error) {
	isPtr := strings.HasPrefix(typeName, "*")
	typeName = strings.TrimPrefix(typeName, "*")

	if info, ok := w.TypeCache[typeName]; ok {
		info.IsPointer = isPtr
		return info, nil
	}

	if types.IsPrimitiveType(typeName) {
		info := &types.TypeInfo{Name: typeName, ImportPath: "builtin", IsPointer: isPtr}
		w.TypeCache[typeName] = info
		return info, nil
	}

	fqn, isLocalAlias := w.findFQNForLocalName(typeName)
	if !isLocalAlias {
		fqn = typeName
	}

	pkgPath, typeNameOnly := splitFQN(fqn)
	if pkgPath == "" {
		pkgPath = w.currentPkg.PkgPath
	}

	pkg := w.findPackage(pkgPath)
	if pkg == nil {
		var err error
		pkg, err = w.loadPackage(pkgPath)
		if err != nil {
			return nil, fmt.Errorf("package %q for type %q not found or loaded", pkgPath, typeName)
		}
	}

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*goast.TypeSpec); ok && typeSpec.Name.Name == typeNameOnly {
						info := w.buildTypeInfo(typeSpec, pkg)
						info.IsPointer = isPtr
						if isLocalAlias {
							info.LocalAlias = typeName
						}
						w.TypeCache[typeName] = info
						return info, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("type %q not found in package %q", typeNameOnly, pkgPath)
}

// GetTypeCache returns the cache of known types.
func (w *PackageWalker) GetTypeCache() map[string]*types.TypeInfo {
	return w.TypeCache
}

// GetLocalTypeNameToFQN returns the map of local type aliases to their FQN.
func (w *PackageWalker) GetLocalTypeNameToFQN() map[string]string {
	return w.localTypeNameToFQN
}

// --- Helper Methods ---

func (w *PackageWalker) buildTypeInfo(spec *goast.TypeSpec, pkg *packages.Package) *types.TypeInfo {
	info := &types.TypeInfo{
		Name:       spec.Name.Name,
		ImportPath: pkg.PkgPath,
	}
	if structType, ok := spec.Type.(*goast.StructType); ok {
		for _, field := range structType.Fields.List {
			if len(field.Names) > 0 && field.Names[0].IsExported() {
				fieldInfo := types.StructField{
					Name:      field.Names[0].Name,
					Type:      w.exprToString(field.Type, pkg),
					IsPointer: isPointer(field.Type),
				}
				if field.Tag != nil {
					fieldInfo.Tags = field.Tag.Value
				}
				info.Fields = append(info.Fields, fieldInfo)
			}
		}
	}
	return info
}

func (w *PackageWalker) findPackage(pkgPath string) *packages.Package {
	for _, p := range w.allKnownPkgs {
		if p.PkgPath == pkgPath {
			return p
		}
	}
	return nil
}

func (w *PackageWalker) findFQNForLocalName(name string) (string, bool) {
	fqn, ok := w.localTypeNameToFQN[name]
	return fqn, ok
}

func (w *PackageWalker) getPackageTypes(pkg *packages.Package) map[string]bool {
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

func (w *PackageWalker) loadPackage(importPath string) (*packages.Package, error) {
	if pkg, ok := w.loadedPkgs[importPath]; ok {
		return pkg, nil
	}
	cfg := &packages.Config{Mode: w.packageMode, Dir: filepath.Dir(w.currentPkg.GoFiles[0])}
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
	w.AddPackages(pkg)
	return pkg, nil
}

func (w *PackageWalker) resolveAlias(aliasOrPath string) string {
	if path, ok := w.pathAliases[aliasOrPath]; ok {
		return path
	}
	return aliasOrPath
}

func (w *PackageWalker) exprToString(expr goast.Expr, pkg *packages.Package) string {
	if pkg == nil || pkg.TypesInfo == nil {
		switch e := expr.(type) {
		case *goast.Ident:
			return e.Name
		case *goast.SelectorExpr:
			return fmt.Sprintf("%s.%s", w.exprToString(e.X, pkg), e.Sel.Name)
		default:
			return "unknown"
		}
	}
	tv := pkg.TypesInfo.TypeOf(expr)
	if tv == nil {
		return "unknown"
	}
	return tv.String()
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
