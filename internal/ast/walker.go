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

	// The qualifier function ensures that we use the full import path for package names.
	qualifier := func(p *gotypes.Package) string {
		// For packages in the standard library, we can just use their path.
		if p.Path() != "" && !strings.Contains(p.Path(), ".") {
			return p.Path()
		}
		// For other packages, search our known packages list.
		for _, knownPkg := range w.allKnownPkgs {
			if knownPkg.Types.Path() == p.Path() {
				return knownPkg.PkgPath
			}
		}
		// Fallback to the package's path if not found.
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
	// 移除指针前缀
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

	// Iterate through each file in the package to collect imports
	for _, file := range pkg.Syntax {
		w.collectImports(file) // Call the existing collectImports for each file
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

// processFileDecls processes all directives in a file, both file-level and type-level.
func (w *PackageWalker) processFileDecls(file *goast.File) error {
	// Use a map to aggregate settings for the same package conversion within a single file.
	pkgConfigs := make(map[string]*types.PackageConversionConfig)

	// 1. Process file-level directives for package-to-package conversion
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			if strings.HasPrefix(comment.Text, "//go:abgen:") {
				// File-level directives are not associated with a type, so typeCfg is nil.
				w.parseAndApplyDirective(comment.Text, nil, pkgConfigs)
			}
		}
	}
	// Add collected package configs to the walker
	for _, cfg := range pkgConfigs {
		if cfg.SourcePackage != "" && cfg.TargetPackage != "" {
			w.PackageConfigs = append(w.PackageConfigs, cfg)
		}
	}

	// 2. Process type-level directives
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*goast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*goast.TypeSpec)
			if !ok {
				continue
			}

			// Record local type aliases and determine the effective SourceType for directives
			var effectiveSourceType string
			if _, isStruct := typeSpec.Type.(*goast.StructType); !isStruct {
				// It's a type alias
				aliasedToType := w.exprToString(typeSpec.Type, w.currentPkg)
				w.localTypeAliases[typeSpec.Name.Name] = aliasedToType
				effectiveSourceType = aliasedToType // Use the underlying type for SourceType
			} else {
				effectiveSourceType = w.currentPkg.PkgPath + "." + typeSpec.Name.Name // Use the fully qualified name for struct types
			}

			// Process comments attached to the TypeSpec
			if typeSpec.Doc != nil {
				var typeCfg *types.ConversionConfig
				for _, comment := range typeSpec.Doc.List {
					if strings.HasPrefix(comment.Text, "//go:abgen:") {
						if typeCfg == nil {
							typeCfg = &types.ConversionConfig{
								SourceType:   effectiveSourceType, // Use the determined effective source type
								IgnoreFields: make(map[string]bool),
								SourcePrefix: "",
								SourceSuffix: "",
								TargetPrefix: "",
								TargetSuffix: "",
							}
						}
						w.parseAndApplyDirective(comment.Text, typeCfg, nil)
					}
				}
				slog.Debug("After processing type-level directives", "type", typeSpec.Name.Name, "typeCfg_SourceType", typeCfg.SourceType, "typeCfg_TargetType", typeCfg.TargetType)
				if typeCfg != nil && typeCfg.TargetType != "" {
					w.AddConversion(typeCfg)
				}
			}
		}
	}
	return nil
}

// parseAndApplyDirective parses a single `//go:abgen:` line and applies it to the relevant config object.
// Directives can be in two forms:
// 1. `//go:abgen:key:subkey="value"`: keyStr is "key:subkey", value is "value".
// 2. `//go:abgen:key:subkey`: keyStr is "key:subkey", value is empty.
func (w *PackageWalker) parseAndApplyDirective(line string, typeCfg *types.ConversionConfig, pkgConfigs map[string]*types.PackageConversionConfig) {
	slog.Debug("parseAndApplyDirective", "line", line)
	directive := strings.TrimPrefix(line, "//go:abgen:")

	keyStr := directive
	value := ""

	if parts := strings.SplitN(directive, "=", 2); len(parts) == 2 {
		keyStr = parts[0]
		value = strings.Trim(parts[1], `"`)
	}

	keys := strings.Split(keyStr, ":")
	if len(keys) < 2 {
		slog.Warn("Invalid directive format: missing verb or subject.", "directive", line)
		return
	}

	verb := keys[0]
	subject := keys[1]
	slog.Debug("Parsed Directive", "verb", verb, "subject", subject, "keys", keys, "value", value)

	switch {
	case verb == "convert" && subject == "package":
		// ... (This logic remains the same as it was correct)
		if pkgConfigs == nil {
			return
		}
		const pkgConfigKey = "pkg"
		if pkgConfigs[pkgConfigKey] == nil {
			pkgConfigs[pkgConfigKey] = &types.PackageConversionConfig{
				IgnoreTypes: make(map[string]bool),
				FieldMap:    make(map[string]string),
			}
		}
		cfg := pkgConfigs[pkgConfigKey]

		if len(keys) == 3 {
			property := keys[2]
			switch property {
			case "source":
				cfg.SourcePackage = value
			case "target":
				cfg.TargetPackage = value
			case "direction":
				cfg.Direction = value
			case "ignore":
				for _, t := range strings.Split(value, ",") {
					cfg.IgnoreTypes[strings.TrimSpace(t)] = true
				}
			}
		} else if len(keys) == 4 {
			entity, property := keys[2], keys[3]
			switch entity {
			case "source":
				if property == "prefix" {
					cfg.SourcePrefix = value
				} else if property == "suffix" {
					cfg.SourceSuffix = value
				}
			case "target":
				if property == "prefix" {
					cfg.TargetPrefix = value
				} else if property == "suffix" {
					cfg.TargetSuffix = value
				}
			}
		}

	case verb == "convert":
		if typeCfg == nil {
			return
		}

		// The ONLY valid subject is 'field' when len(keys) > 2 is not met.
		// All other directives must have len(keys) == 2.
		if subject == "field" {
			// CORRECT FORMAT: //go:abgen:convert:field="FieldName1:CustomFunc1,FieldName2:CustomFunc2"
			// The rule is 'convert:field', and all user-defined values are AFTER the '='.
			if len(keys) != 2 {
				slog.Warn("Invalid 'convert:field' directive. The field name must be specified in the value part.", "directive", line)
				return
			}
			mappings := strings.Split(value, ",")
			if typeCfg.TypeConversionRules == nil {
				typeCfg.TypeConversionRules = make([]types.TypeConversionRule, 0)
			}

			// Resolve main source and target types to get their field information
			mainSourceTypeInfo, err := w.Resolve(typeCfg.SourceType)
			if err != nil {
				slog.Warn("Could not resolve main source type for field directive", "mainSourceType", typeCfg.SourceType, "error", err)
				return
			}
			mainTargetTypeInfo, err := w.Resolve(typeCfg.TargetType)
			if err != nil {
				slog.Warn("Could not resolve main target type for field directive", "mainTargetType", typeCfg.TargetType, "error", err)
				return
			}

			for _, mapping := range mappings {
				parts := strings.SplitN(strings.TrimSpace(mapping), ":", 3)
				if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
					slog.Warn("Invalid mapping in 'convert:field' directive. Expected 'SourceType:TargetType:ConvertFunc'.", "mapping", mapping)
					continue
				}

				sourceType := parts[0]
				targetType := parts[1]
				convertFunc := parts[2]

				var resolvedSourceType string
				if types.IsPrimitiveType(sourceType) || strings.Contains(sourceType, "/") || strings.Contains(sourceType, ".") {
					resolvedSourceType = sourceType // Already primitive or fully qualified
				} else {
					// Try to find the field type in the main source struct (typeCfg.SourceType)
					found := false
					for _, field := range mainSourceTypeInfo.Fields {
						// The 'sourceType' in directive refers to the FIELD'S TYPE NAME, not field name
						// So we compare 'sourceType' (e.g., "Gender") with the base name of 'field.Type'
						if strings.HasSuffix(field.Type, sourceType) { // Basic check for now
							resolvedSourceType = field.Type
							found = true
							break
						}
					}
					if !found {
						// If not found as a field type, attempt to resolve globally
						srcInfo := w.resolveTargetType(sourceType)
						if srcInfo.ImportPath != "" && srcInfo.Name != "" {
							resolvedSourceType = srcInfo.ImportPath + "." + srcInfo.Name
						} else {
							resolvedSourceType = sourceType // Fallback if still unresolved
						}
					}
				}

				var resolvedTargetType string
				if types.IsPrimitiveType(targetType) || strings.Contains(targetType, "/") || strings.Contains(targetType, ".") {
					resolvedTargetType = targetType // Already primitive or fully qualified
				} else {
					// Try to find the field type in the main target struct (typeCfg.TargetType)
					found := false
					for _, field := range mainTargetTypeInfo.Fields {
						if strings.HasSuffix(field.Type, targetType) { // Basic check for now
							resolvedTargetType = field.Type
							found = true
							break
						}
					}
					if !found {
						// If not found as a field type, attempt to resolve globally
						dstInfo := w.resolveTargetType(targetType)
						if dstInfo.ImportPath != "" && dstInfo.Name != "" {
							resolvedTargetType = dstInfo.ImportPath + "." + dstInfo.Name
						} else {
							resolvedTargetType = targetType // Fallback if still unresolved
						}
					}
				}

				rule := types.TypeConversionRule{
					SourceTypeName: resolvedSourceType,
					TargetTypeName: resolvedTargetType,
					ConvertFunc:    convertFunc,
				}
				typeCfg.TypeConversionRules = append(typeCfg.TypeConversionRules, rule)
				slog.Debug("Applied Directive: Custom Field Conversion Rule", "source", rule.SourceTypeName, "target", rule.TargetTypeName, "func", rule.ConvertFunc)
			}
		} else if len(keys) == 2 {
			// Handles: convert:target, convert:direction, convert:ignore
			switch subject {
			case "target":
				typeCfg.TargetType = value
			case "direction":
				typeCfg.Direction = value
			case "ignore":
				for _, f := range strings.Split(value, ",") {
					typeCfg.IgnoreFields[strings.TrimSpace(f)] = true
				}
			}
		} else if len(keys) == 3 {
			// Handles: convert:source:suffix, convert:target:prefix etc.
			property := keys[2]
			switch subject {
			case "source":
				if property == "prefix" {
					typeCfg.SourcePrefix = value
				} else if property == "suffix" {
					typeCfg.SourceSuffix = value
				}
			case "target":
				if property == "prefix" {
					typeCfg.TargetPrefix = value
				} else if property == "suffix" {
					typeCfg.TargetSuffix = value
				}
			}
		}
	}
}

// resolveTargetType 解析目标类型
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
		// slog.Debug("resolveTargetType: 检查包", "PkgPath", pkg.PkgPath) // Too verbose
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
			// slog.Debug("resolveTargetType: 检查已知包", "p.PkgPath", p.PkgPath) // Too verbose
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
					// slog.Debug("resolveTargetType: 检查已知包 (通过别名)", "p.PkgPath", p.PkgPath) // Too verbose
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
			// In generated code, we might not need to specify it if it doesn't conflict.
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

// parseStructFields 解析结构体字段
func (w *PackageWalker) parseStructFields(typeSpec *goast.TypeSpec, pkg *packages.Package) types.TypeInfo {
	slog.Debug("parseStructFields: 解析结构体字段", "类型名", typeSpec.Name.Name, "包名", pkg.Name, "包路径", pkg.PkgPath)
	info := types.TypeInfo{
		Name:        typeSpec.Name.Name,
		PkgName:     pkg.Name,
		ImportPath:  pkg.PkgPath,
		ImportAlias: w.findAliasForPath(pkg.PkgPath), // Find and set the import alias
		Fields:      []types.StructField{},
	}

	// 检查是否为结构体类型
	if structType, ok := typeSpec.Type.(*goast.StructType); ok {
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				// Handle embedded fields
				if ident, ok := field.Type.(*goast.Ident); ok {
					slog.Debug("处理内嵌字段", "类型", ident.Name)
					embeddedInfo := w.resolveTargetType(ident.Name)
					info.Fields = append(info.Fields, embeddedInfo.Fields...)
				} else if selExpr, ok := field.Type.(*goast.SelectorExpr); ok {
					embeddedTypeName := w.exprToString(selExpr, pkg)
					slog.Debug("处理跨包内嵌字段", "类型", embeddedTypeName)
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

// IsStructOrStructAlias 检查类型是否为结构体或结构体别名（公开方法）
func (w *PackageWalker) IsStructOrStructAlias(typeSpec *goast.TypeSpec) bool {
	return w.isStructOrStructAlias(typeSpec)
}

// isStructOrStructAlias 检查类型是否为结构体或结构体别名
func (w *PackageWalker) isStructOrStructAlias(typeSpec *goast.TypeSpec) bool {
	// 直接是结构体
	if _, ok := typeSpec.Type.(*goast.StructType); ok {
		return true
	}

	// 检查是否为结构体别名
	if ident, ok := typeSpec.Type.(*goast.Ident); ok {
		// 解析别名的目标类型
		targetInfo := w.resolveTargetType(ident.Name)
		// 如果目标类型有字段，说明是结构体
		return len(targetInfo.Fields) > 0
	}

	// 检查是否为跨包的结构体别名
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

// collectImports 收集导入信息
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

		// 去重处理
		if existing, exists := w.imports[alias]; exists {
			slog.Debug("包别名冲突", "别名", alias, "现有路径", existing, "新路径", realPath)
			continue
		}

		w.imports[alias] = realPath
		slog.Info("记录导入", "别名", alias, "路径", realPath)
	}
}

// 辅助方法
func (w *PackageWalker) ensureNodeExists(typeName string) {
	if _, exists := w.graph[typeName]; !exists {
		w.graph[typeName] = &types.ConversionNode{
			Configs: make(map[string]*types.ConversionConfig),
		}
	}
}

// AddConversion 核心处理方法
func (w *PackageWalker) AddConversion(cfg *types.ConversionConfig) {
	slog.Debug("AddConversion", "SourceType", cfg.SourceType, "TargetType", cfg.TargetType, "Direction", cfg.Direction)
	// 初始化节点逻辑
	w.ensureNodeExists(cfg.SourceType)
	w.ensureNodeExists(cfg.TargetType)

	// 配置存储逻辑
	configKey := cfg.SourceType + "2" + cfg.TargetType
	graphNode := w.graph[cfg.SourceType]
	graphNode.Configs[configKey] = cfg

			// 方向处理逻辑
		switch cfg.Direction {
		case "to":
			graphNode.ToConversions = AppendIfNotExists(graphNode.ToConversions, cfg.TargetType)
		case "from":
			// 'from' means cfg.SourceType can be converted FROM cfg.TargetType.
			// So cfg.TargetType becomes the source for conversion TO cfg.SourceType.
			// We need to ensure the target type node exists to record this.
			w.ensureNodeExists(cfg.TargetType)
			targetNode := w.graph[cfg.TargetType]
			targetNode.FromConversions = AppendIfNotExists(targetNode.FromConversions, cfg.SourceType)
	
			// Also, register a conversion config from Target to Source.
			// Create a new config for the reverse direction in the graph.
			reverseCfg := *cfg // Create a copy
			reverseCfg.SourceType = cfg.TargetType
			reverseCfg.TargetType = cfg.SourceType
			reverseCfg.Direction = "to" // The reversed config is now a "to" conversion from its new source.
	
			// Swap prefixes/suffixes for the reverse config
			reverseCfg.SourcePrefix = cfg.TargetPrefix
			reverseCfg.SourceSuffix = cfg.TargetSuffix
			reverseCfg.TargetPrefix = cfg.SourcePrefix
			reverseCfg.TargetSuffix = cfg.SourceSuffix
	
			reverseConfigKey := reverseCfg.SourceType + "2" + reverseCfg.TargetType
			if targetNode.Configs == nil {
				targetNode.Configs = make(map[string]*types.ConversionConfig)
			}
			targetNode.Configs[reverseConfigKey] = &reverseCfg
	
		default: // "both"
			// Handle SourceType -> TargetType
			graphNode.ToConversions = AppendIfNotExists(graphNode.ToConversions, cfg.TargetType)
	
			// Handle TargetType -> SourceType (the reverse conversion)
			w.ensureNodeExists(cfg.TargetType)
			targetNode := w.graph[cfg.TargetType]
			targetNode.FromConversions = AppendIfNotExists(targetNode.FromConversions, cfg.SourceType)
	
			// Explicitly create and store a ConversionConfig for the reverse direction
			reverseCfg := *cfg // Create a copy of the original config
			reverseCfg.SourceType = cfg.TargetType
			reverseCfg.TargetType = cfg.SourceType
			reverseCfg.Direction = "to" // The reversed config is now a "to" conversion from its new source.
	
			// Swap prefixes/suffixes for the reverse config
			reverseCfg.SourcePrefix = cfg.TargetPrefix
			reverseCfg.SourceSuffix = cfg.TargetSuffix
			reverseCfg.TargetPrefix = cfg.SourcePrefix
			reverseCfg.TargetSuffix = cfg.SourceSuffix
	
			reverseConfigKey := reverseCfg.SourceType + "2" + reverseCfg.TargetType
			if targetNode.Configs == nil {
				targetNode.Configs = make(map[string]*types.ConversionConfig)
			}
			targetNode.Configs[reverseConfigKey] = &reverseCfg
		}
		// For "to" and "both" directions, the original config (SourceType -> TargetType) is handled implicitly by graphNode.Configs[configKey] = cfg
		// For "from" direction, we explicitly created and added a reversed config.
		// We need to make sure that the original config (SourceType -> TargetType) is NOT stored if the direction is "from".
		if cfg.Direction == "from" {
			delete(graphNode.Configs, configKey)
		}}
