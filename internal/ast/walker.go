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

// PackageWalker 包遍历器
type PackageWalker struct {
	imports          map[string]string
	graph            types.ConversionGraph
	currentPkg       *packages.Package
	typeCache        map[string]types.TypeInfo
	loadedPkgs       map[string]*packages.Package // 缓存已加载的包
	packageMode      packages.LoadMode
	PackageConfigs   []*types.PackageConversionConfig
	allKnownPkgs     []*packages.Package // New field to hold all packages known to the resolver
	localTypeAliases map[string]string   // New: to track local type aliases
}

// AddKnownPackages adds more *packages.Package instances to the walker's known packages.
func (w *PackageWalker) AddKnownPackages(pkgs ...*packages.Package) {
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

// exprToString resolves an expression to its fully qualified type name.
func (w *PackageWalker) exprToString(expr goast.Expr, pkg *packages.Package) string {
	if pkg == nil || pkg.TypesInfo == nil {
		return fmt.Sprintf("<missing_type_info_for_%T>", expr)
	}

	tv, ok := pkg.TypesInfo.Types[expr]
	if !ok || tv.Type == nil {
		if ident, isIdent := expr.(*goast.Ident); isIdent {
			return ident.Name
		}
		return fmt.Sprintf("<unresolved_type_%T>", expr)
	}

	qualifier := func(p *gotypes.Package) string {
		for _, knownPkg := range w.allKnownPkgs {
			if knownPkg.Types.Path() == p.Path() {
				return knownPkg.PkgPath
			}
		}
		return p.Path()
	}

	return gotypes.TypeString(tv.Type, qualifier)
}

func (w *PackageWalker) GetTypeCache() map[string]types.TypeInfo {
	return w.typeCache
}

func (w *PackageWalker) GetImports() map[string]string {
	return w.imports
}

func (w *PackageWalker) GetLocalTypeAliases() map[string]string {
	return w.localTypeAliases
}

// NewPackageWalker 创建新的遍历器
func NewPackageWalker(graph types.ConversionGraph) *PackageWalker {
	return &PackageWalker{
		graph:            graph,
		imports:          make(map[string]string),
		typeCache:        make(map[string]types.TypeInfo),
		loadedPkgs:       make(map[string]*packages.Package),
		packageMode:      packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		PackageConfigs:   make([]*types.PackageConversionConfig, 0),
		localTypeAliases: make(map[string]string),
	}
}

// Resolve is a public method, for resolving type information
func (w *PackageWalker) Resolve(typeName string) (types.TypeInfo, error) {
	typeName = strings.TrimPrefix(typeName, "*")
	info := w.resolveTargetType(typeName)
	if info.Name == "" {
		return types.TypeInfo{}, fmt.Errorf("type %q not found", typeName)
	}
	return info, nil
}

// WalkPackage 遍历包内的类型定义
func (w *PackageWalker) WalkPackage(pkg *packages.Package) error {
	slog.Info("开始遍历包", "包", pkg.PkgPath)
	w.currentPkg = pkg

	for _, file := range pkg.Syntax {
		w.collectImports(file)
	}

	for _, file := range pkg.Syntax {
		filename := pkg.Fset.File(file.Pos()).Name()
		slog.Info("遍历文件", "文件", filename)
		if strings.HasSuffix(filepath.Base(filename), ".gen.go") {
			continue
		}
		if err := w.processFileDecls(file); err != nil {
			return err
		}
	}
	return nil
}

func (w *PackageWalker) processFileDecls(file *goast.File) error {
	// Pass 1: Collect all type aliases in the file first.
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*goast.TypeSpec); ok {
					if _, isStruct := typeSpec.Type.(*goast.StructType); !isStruct {
						aliasedToType := w.exprToString(typeSpec.Type, w.currentPkg)
						w.localTypeAliases[typeSpec.Name.Name] = aliasedToType
					}
				}
			}
		}
	}

	// Pass 2: Find directive groups and process them.
	for _, commentGroup := range file.Comments {
		var definingDirective string
		var modifierDirectives []string
		isPackagePair := false
		isDirectiveGroup := false

		for _, comment := range commentGroup.List {
			line := comment.Text
			if !strings.HasPrefix(line, "//go:abgen:") {
				continue
			}
			isDirectiveGroup = true
			directive := strings.TrimPrefix(line, "//go:abgen:")
			if strings.HasPrefix(directive, "pair:packages=") {
				definingDirective = line
				isPackagePair = true
			} else if strings.HasPrefix(directive, "convert=") {
				definingDirective = line
			} else {
				modifierDirectives = append(modifierDirectives, line)
			}
		}

		if !isDirectiveGroup || definingDirective == "" {
			continue
		}

		if isPackagePair {
			pkgCfg := &types.PackageConversionConfig{
				IgnoreTypes:         make(map[string]bool),
				IgnoreFields:        make(map[string]bool),
				RemapFields:         make(map[string]string),
				TypeConversionRules: make([]types.TypeConversionRule, 0),
			}
			tempPkgConfigs := map[string]*types.PackageConversionConfig{"pkg-pair": pkgCfg}
			w.parseAndApplyDirective(definingDirective, nil, tempPkgConfigs)
			for _, mod := range modifierDirectives {
				w.parseAndApplyDirective(mod, nil, tempPkgConfigs)
			}
			if pkgCfg.SourcePackage != "" && pkgCfg.TargetPackage != "" {
				w.PackageConfigs = append(w.PackageConfigs, pkgCfg)
			}
		} else { // This is a type-level convert config
			var associatedDecl *goast.GenDecl
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*goast.GenDecl)
				if !ok || genDecl.Tok != token.TYPE {
					continue
				}
				doc := genDecl.Doc
				if doc == nil && len(genDecl.Specs) > 0 {
					if spec, ok := genDecl.Specs[0].(*goast.TypeSpec); ok {
						doc = spec.Doc
					}
				}
				if doc != nil && doc.Pos() == commentGroup.Pos() {
					associatedDecl = genDecl
					break
				}
			}

			if associatedDecl == nil {
				continue
			}

			for _, spec := range associatedDecl.Specs {
				typeSpec, ok := spec.(*goast.TypeSpec)
				if !ok {
					continue
				}
				var effectiveSourceType string
				if aliasedTo, isAlias := w.localTypeAliases[typeSpec.Name.Name]; isAlias {
					effectiveSourceType = aliasedTo
				} else {
					effectiveSourceType = w.currentPkg.PkgPath + "." + typeSpec.Name.Name
				}
				typeCfg := &types.ConversionConfig{
					Source:       &types.EndpointConfig{Type: effectiveSourceType},
					Target:       &types.EndpointConfig{},
					IgnoreFields: make(map[string]bool),
					RemapFields:  make(map[string]string),
					TypeConversionRules: make([]types.TypeConversionRule, 0),
				}
				w.parseAndApplyDirective(definingDirective, typeCfg, nil)
				for _, mod := range modifierDirectives {
					w.parseAndApplyDirective(mod, typeCfg, nil)
				}
				if typeCfg.Target.Type != "" {
					w.AddConversion(typeCfg)
				}
			}
		}
	}
	return nil
}

func (w *PackageWalker) parseAndApplyDirective(line string, typeCfg *types.ConversionConfig, pkgConfigs map[string]*types.PackageConversionConfig) {
	directive := strings.TrimPrefix(line, "//go:abgen:")
	keyStr, value, _ := strings.Cut(directive, "=")
	value = strings.Trim(value, `"`)
	keys := strings.Split(keyStr, ":")
	verb := keys[0]

	if typeCfg == nil { // File-level
		const pkgConfigKey = "pkg-pair"
		if pkgConfigs == nil || pkgConfigs[pkgConfigKey] == nil {
			return
		}
		cfg := pkgConfigs[pkgConfigKey]
		switch verb {
		case "pair":
			if len(keys) > 1 && keys[1] == "packages" {
				paths := strings.Split(value, ",")
				if len(paths) == 2 {
					cfg.SourcePackage = strings.TrimSpace(paths[0])
					cfg.TargetPackage = strings.TrimSpace(paths[1])
				}
			}
		case "convert":
			if len(keys) == 2 {
				switch keys[1] {
				case "direction":
					cfg.Direction = value
				case "ignore":
					for _, f := range strings.Split(value, ",") {
						cfg.IgnoreFields[strings.TrimSpace(f)] = true
					}
				case "remap": // File-level remap
					parts := strings.SplitN(value, ":", 2)
					if len(parts) == 2 {
						cfg.RemapFields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				case "rule":
					rule := parseRule(value, w)
					if rule.SourceTypeName != "" && rule.TargetTypeName != "" && rule.ConvertFunc != "" {
						cfg.TypeConversionRules = append(cfg.TypeConversionRules, rule)
					}
				}
			} else if len(keys) == 3 {
				subject, property := keys[1], keys[2]
				switch subject {
				case "source":
					if property == "suffix" {
						cfg.SourceSuffix = value
					} else if property == "prefix" {
						cfg.SourcePrefix = value
					}
				case "target":
					if property == "suffix" {
						cfg.TargetSuffix = value
					} else if property == "prefix" {
						cfg.TargetPrefix = value
					}
				}
			}
		}
		return
	}

	// Type-level
	if verb == "convert" {
		if len(keys) == 1 { // convert="A,B"
			parts := strings.Split(value, ",")
			if len(parts) == 2 {
				targetName := strings.TrimSpace(parts[1])
				if aliasedTarget, ok := w.localTypeAliases[targetName]; ok {
					typeCfg.Target.Type = aliasedTarget
				} else {
					typeCfg.Target.Type = targetName
				}
			}
		} else if len(keys) == 2 { // convert:ignore="..."
			switch keys[1] {
			case "direction":
				typeCfg.Direction = value
			case "ignore":
				for _, f := range strings.Split(value, ",") {
					typeCfg.IgnoreFields[strings.TrimSpace(f)] = true
				}
			case "remap": // Type-level remap
				parts := strings.SplitN(value, ":", 2)
				if len(parts) == 2 {
					typeCfg.RemapFields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			case "rule":
				rule := parseRule(value, w)
				if rule.SourceTypeName != "" && rule.TargetTypeName != "" && rule.ConvertFunc != "" {
					typeCfg.TypeConversionRules = append(typeCfg.TypeConversionRules, rule)
				}
			}
		} else if len(keys) == 3 {
			subject, property := keys[1], keys[2]
			switch subject {
			case "source":
				if property == "suffix" {
					typeCfg.Source.Suffix = value
				} else if property == "prefix" {
					typeCfg.Source.Prefix = value
				}
			case "target":
				if property == "suffix" {
					typeCfg.Target.Suffix = value
				} else if property == "prefix" {
					typeCfg.Target.Prefix = value
				}
			}
		}
	}
}

func parseRule(value string, walker *PackageWalker) types.TypeConversionRule {
	var rule types.TypeConversionRule
	ruleParts := strings.Split(value, ",")
	for _, part := range ruleParts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			switch strings.TrimSpace(kv[0]) {
			case "source":
				resolvedType, err := walker.Resolve(strings.TrimSpace(kv[1]))
				if err != nil {
					slog.Warn("Failed to resolve source type in rule", "type", strings.TrimSpace(kv[1]), "error", err)
					rule.SourceTypeName = strings.TrimSpace(kv[1]) // Fallback
				} else {
					rule.SourceTypeName = resolvedType.ImportPath + "." + resolvedType.Name
				}
			case "target":
				resolvedType, err := walker.Resolve(strings.TrimSpace(kv[1]))
				if err != nil {
					slog.Warn("Failed to resolve target type in rule", "type", strings.TrimSpace(kv[1]), "error", err)
					rule.TargetTypeName = strings.TrimSpace(kv[1]) // Fallback
				} else {
					rule.TargetTypeName = resolvedType.ImportPath + "." + resolvedType.Name
				}
			case "func":
				rule.ConvertFunc = strings.TrimSpace(kv[1])
			}
		}
	}
	return rule
}

// resolveTargetType resolves the TypeInfo for a given type name, which may be in an external package.
func (w *PackageWalker) resolveTargetType(targetType string) types.TypeInfo {
	slog.Debug("resolveTargetType: 开始解析目标类型", "targetType", targetType)
	// 1. Check cache first
	if info, exists := w.typeCache[targetType]; exists {
		slog.Debug("resolveTargetType: 从缓存中找到", "targetType", targetType, "info.Name", info.Name, "info.ImportPath", info.ImportPath)
		return info
	}

	// 立即将当前类型添加到缓存中，防止循环引用导致死循环
	// 先创建一个占位符，稍后更新实际内容
	w.typeCache[targetType] = types.TypeInfo{Name: targetType}
	slog.Debug("resolveTargetType: 添加占位符到缓存", "targetType", targetType)

	// 2. Handle local aliases within all known packages
	slog.Debug("resolveTargetType: 检查所有已知包中的本地别名", "targetType", targetType)
	for _, pkg := range w.allKnownPkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
					for _, spec := range genDecl.Specs {
						if typeSpec, ok := spec.(*goast.TypeSpec); ok && typeSpec.Name.Name == targetType {
							slog.Debug("resolveTargetType: 在已知包中找到本地别名定义", "pkg", pkg.PkgPath, "alias", targetType)
							if aliasIdent, aliasOk := typeSpec.Type.(*goast.Ident); aliasOk {
								slog.Debug("resolveTargetType: 别名指向同一包内的类型 (Ident)", "alias", targetType, "targetIdent", aliasIdent.Name)
								resolvedInfo := w.resolveTargetType(aliasIdent.Name) // RECURSIVE CALL
								w.typeCache[targetType] = resolvedInfo               // Cache the final resolved type under the original alias name
								slog.Debug("resolveTargetType: 递归调用返回 (Ident)", "targetType", targetType, "resolvedName", resolvedInfo.Name, "resolvedPath", resolvedInfo.ImportPath)
								return resolvedInfo
							} else if aliasSelector, aliasOk := typeSpec.Type.(*goast.SelectorExpr); aliasOk {
								aliasedTypeName := w.exprToString(aliasSelector, pkg)
								slog.Debug("resolveTargetType: 别名指向导入类型 (SelectorExpr)", "alias", targetType, "targetQualified", aliasedTypeName)
								resolvedInfo := w.resolveTargetType(aliasedTypeName) // RECURSIVE CALL
								w.typeCache[targetType] = resolvedInfo               // Cache the final resolved type under the original alias name
								slog.Debug("resolveTargetType: 递归调用返回 (SelectorExpr)", "targetType", targetType, "resolvedName", resolvedInfo.Name, "resolvedPath", resolvedInfo.ImportPath)
								return resolvedInfo
							}
						}
					}
				}
			}
		}
	}

	// 3. Handle qualified types (e.g., "ent.User" or "some/path/to/pkg.MyType")
	parts := strings.Split(targetType, ".")
	if len(parts) > 1 {
		pkgIdentifier := strings.Join(parts[:len(parts)-1], ".") // Handle full path like "a/b/c.Type"
		typeName := parts[len(parts)-1]
		slog.Debug("resolveTargetType: 处理限定类型", "pkgIdentifier", pkgIdentifier, "typeName", typeName)

		var foundPkg *packages.Package

		// First, try to find the package by its full PkgPath if pkgIdentifier is a full path
		slog.Debug("resolveTargetType: 尝试通过完整包路径查找", "pkgIdentifier", pkgIdentifier)
		for _, p := range w.allKnownPkgs {
			if p.PkgPath == pkgIdentifier {
				foundPkg = p
				slog.Debug("resolveTargetType: 找到匹配的已知包 (通过完整路径)", "foundPkg.PkgPath", foundPkg.PkgPath, "foundPkg.Name", foundPkg.Name)
				break
			}
		}

		// If not found by full PkgPath, try to resolve via imports (alias)
		if foundPkg == nil {
			slog.Debug("resolveTargetType: 未通过完整路径找到，尝试通过导入别名查找", "pkgIdentifier", pkgIdentifier)
			if importPath, exists := w.imports[pkgIdentifier]; exists {
				slog.Debug("resolveTargetType: 找到导入别名", "alias", pkgIdentifier, "importPath", importPath)
				for _, p := range w.allKnownPkgs {
					if p.PkgPath == importPath {
						foundPkg = p
						slog.Debug("resolveTargetType: 找到匹配的已知包 (通过别名)", "foundPkg.PkgPath", foundPkg.PkgPath, "foundPkg.Name", foundPkg.Name)
						break
					}
				}
			}
		}

		if foundPkg != nil {
			slog.Debug("resolveTargetType: 在找到的包中查找类型", "foundPkg.PkgPath", foundPkg.PkgPath, "typeName", typeName)
			// 5. Find the type spec in the found package
			for _, file := range foundPkg.Syntax {
				for _, decl := range file.Decls {
					if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
						for _, spec := range genDecl.Specs {
							if typeSpec, ok := spec.(*goast.TypeSpec); ok && typeSpec.Name.Name == typeName {
								slog.Debug("resolveTargetType: 成功找到 TypeSpec", "typeName", typeName, "pkgPath", foundPkg.PkgPath)
								// 6. Parse and return its struct info
								info := w.parseStructFields(typeSpec, foundPkg)
								w.typeCache[targetType] = info // Cache the result with original targetType
								slog.Debug("resolveTargetType: 返回 TypeInfo", "typeName", info.Name, "pkgName", info.PkgName, "importPath", info.ImportPath)
								return info
							}
						}
					}
				}
			}
			slog.Debug("resolveTargetType: 未在找到的包的Syntax中找到TypeSpec", "typeName", typeName, "foundPkg.PkgPath", foundPkg.PkgPath)
		} else {
			slog.Debug("resolveTargetType: 未在已知包中找到限定类型所属的包，尝试动态加载", "pkgIdentifier", pkgIdentifier, "typeName", typeName)
			// 尝试动态加载包
			if strings.Contains(pkgIdentifier, "/") {
				slog.Debug("resolveTargetType: 尝试动态加载包", "pkgPath", pkgIdentifier)
				if loadedPkg, err := w.loadPackage(pkgIdentifier); err == nil {
					slog.Debug("resolveTargetType: 成功动态加载包", "pkgPath", loadedPkg.PkgPath, "pkgName", loadedPkg.Name)
					foundPkg = loadedPkg
					w.allKnownPkgs = append(w.allKnownPkgs, loadedPkg)
					// 重新查找类型
					for _, file := range foundPkg.Syntax {
						for _, decl := range file.Decls {
							if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
								for _, spec := range genDecl.Specs {
									if typeSpec, ok := spec.(*goast.TypeSpec); ok && typeSpec.Name.Name == typeName {
										slog.Debug("resolveTargetType: 在动态加载的包中找到类型", "typeName", typeName)
										info := w.parseStructFields(typeSpec, foundPkg)
										w.typeCache[targetType] = info
										slog.Debug("resolveTargetType: 返回动态加载包中的TypeInfo", "typeName", info.Name, "pkgName", info.PkgName, "importPath", info.ImportPath)
										return info
									}
								}
							}
						}
					}
				} else {
					slog.Debug("resolveTargetType: 动态加载包失败", "pkgPath", pkgIdentifier, "error", err)
				}
			}
		}
	}

	// If we reach here, the type was not found. Return an empty TypeInfo.
	slog.Debug("resolveTargetType: 未找到类型，返回空的TypeInfo", "targetType", targetType)
	delete(w.typeCache, targetType) // Remove the placeholder
	return types.TypeInfo{}
}

// findAliasForPath finds the alias used for a given import path in the current context.
func (w *PackageWalker) findAliasForPath(importPath string) string {
	for alias, path := range w.imports {
		if path == importPath {
			// If the alias is the same as the last part of the path, it's an implicit alias.
			// In generated code, we might not need to specify it if it's not conflicting.
			// However, for explicit aliases (like `typespb`), we must use them.
			pathParts := strings.Split(path, "/")
			if alias != "." && alias != "_" && alias != pathParts[len(pathParts)-1] {
				return alias
			}
		}
	}
	// If no explicit alias is found, return the package's own name,
	// which is the default behavior of Go imports.
	if pkg, ok := w.loadedPkgs[importPath]; ok {
		return pkg.Name
	}
	// Fallback if package not loaded (should be rare)
	parts := strings.Split(importPath, "/")
	return parts[len(parts)-1]
}

// parseStructFields parses the fields of a given go/ast.TypeSpec that represents a struct.
func (w *PackageWalker) parseStructFields(typeSpec *goast.TypeSpec, pkg *packages.Package) types.TypeInfo {
	slog.Debug("parseStructFields: 解析结构体字段", "类型名", typeSpec.Name.Name, "包名", pkg.Name, "包路径", pkg.PkgPath)
	info := types.TypeInfo{
		Name:        typeSpec.Name.Name,
		PkgName:     pkg.Name,
		ImportPath:  pkg.PkgPath,
		ImportAlias: w.findAliasForPath(pkg.PkgPath),
		Fields:      []types.StructField{},
	}

	if structType, ok := typeSpec.Type.(*goast.StructType); ok {
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 { // Embedded field
				var embeddedTypeExpr goast.Expr
				if ident, ok := field.Type.(*goast.Ident); ok {
					embeddedTypeExpr = ident
				} else if selExpr, ok := field.Type.(*goast.SelectorExpr); ok {
					embeddedTypeExpr = selExpr
				}
				if embeddedTypeExpr != nil {
					embeddedTypeName := w.exprToString(embeddedTypeExpr, pkg)
					embeddedInfo := w.resolveTargetType(embeddedTypeName)
					info.Fields = append(info.Fields, embeddedInfo.Fields...)
				}
				continue
			}

			fieldName := field.Names[0].Name
			if goast.IsExported(fieldName) {
				info.Fields = append(info.Fields, types.StructField{
					Name:     fieldName,
					Type:     w.exprToString(field.Type, pkg),
					Exported: true,
				})
			}
		}
	}
	return info
}

// IsStructOrStructAlias checks if the given type spec is a struct or an alias to a struct.
func (w *PackageWalker) IsStructOrStructAlias(typeSpec *goast.TypeSpec) bool {
	if _, ok := typeSpec.Type.(*goast.StructType); ok {
		return true
	}
	if ident, ok := typeSpec.Type.(*goast.Ident); ok {
		targetInfo := w.resolveTargetType(ident.Name)
		return len(targetInfo.Fields) > 0
	}
	if selector, ok := typeSpec.Type.(*goast.SelectorExpr); ok {
		typeName := w.exprToString(selector, w.currentPkg)
		targetInfo := w.resolveTargetType(typeName)
		return len(targetInfo.Fields) > 0
	}
	return false
}

// loadPackage loads a package by its import path, using a cache to avoid redundant loads.
func (w *PackageWalker) loadPackage(importPath string) (*packages.Package, error) {
	if pkg, ok := w.loadedPkgs[importPath]; ok {
		return pkg, nil
	}
	cfg := &packages.Config{Mode: w.packageMode}
	pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load package %s: %w", importPath, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no package found for import path %s", importPath)
	}
	w.loadedPkgs[importPath] = pkgs[0]
	return pkgs[0], nil
}

// collectImports collects import statements from a go/ast.File and populates the walker's imports map.
func (w *PackageWalker) collectImports(file *goast.File) {
	for _, imp := range file.Imports {
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			parts := strings.Split(strings.Trim(imp.Path.Value, `"`), "/")
			alias = parts[len(parts)-1]
		}
		realPath := strings.Trim(imp.Path.Value, `"`)
		if _, exists := w.imports[alias]; !exists {
			w.imports[alias] = realPath
		}
	}
}

// ensureNodeExists ensures a ConversionNode exists for a given type name in the conversion graph.
func (w *PackageWalker) ensureNodeExists(typeName string) {
	if _, exists := w.graph[typeName]; !exists {
		w.graph[typeName] = &types.ConversionNode{
			Configs: make(map[string]*types.ConversionConfig),
		}
	}
}



// AddConversion adds a new ConversionConfig to the conversion graph, handling bidirectional conversions.
func (w *PackageWalker) AddConversion(cfg *types.ConversionConfig) {
	w.ensureNodeExists(cfg.Source.Type)
	w.ensureNodeExists(cfg.Target.Type)

	if cfg.Direction == "" {
		cfg.Direction = "both"
	}
	
	configKey := cfg.Source.Type + "2" + cfg.Target.Type

	if cfg.Direction == "to" || cfg.Direction == "both" {
		w.graph[cfg.Source.Type].ToConversions = AppendIfNotExists(w.graph[cfg.Source.Type].ToConversions, cfg.Target.Type)
		w.graph[cfg.Source.Type].Configs[configKey] = cfg
	}

	if cfg.Direction == "from" || cfg.Direction == "both" {
		w.graph[cfg.Target.Type].ToConversions = AppendIfNotExists(w.graph[cfg.Target.Type].ToConversions, cfg.Source.Type)
		
		reverseCfg := &types.ConversionConfig{
			Source: &types.EndpointConfig{
				Type:   cfg.Target.Type,
				Prefix: cfg.Target.Prefix,
				Suffix: cfg.Target.Suffix,
			},
			Target: &types.EndpointConfig{
				Type:   cfg.Source.Type,
				Prefix: cfg.Source.Prefix,
				Suffix: cfg.Source.Suffix,
			},
			Direction:           "to", // The reverse is always a simple "to"
			IgnoreFields:        cfg.IgnoreFields,
			RemapFields:         cfg.RemapFields, // Note: remap is usually one-way, might need adjustment
			TypeConversionRules: cfg.TypeConversionRules,
		}
		
		reverseKey := reverseCfg.Source.Type + "2" + reverseCfg.Target.Type
		w.graph[reverseCfg.Source.Type].Configs[reverseKey] = reverseCfg
	}
}