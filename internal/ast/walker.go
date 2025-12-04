// Package ast implements the functions, types, and interfaces for the module.
package ast

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"log/slog"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/types"
)

// PackageWalker 包遍历器
type PackageWalker struct {
	imports        map[string]string
	graph          types.ConversionGraph
	currentPkg     *packages.Package
	typeCache      map[string]types.TypeInfo
	loadedPkgs     map[string]*packages.Package // 缓存已加载的包
	packageMode    packages.LoadMode
	PackageConfigs []*types.PackageConversionConfig
	allKnownPkgs   []*packages.Package // New field to hold all packages known to the resolver
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

// 修改 exprToString 方法，确保与 resolver 的逻辑一致
func (w *PackageWalker) exprToString(expr goast.Expr) string {
	switch e := expr.(type) {
	case *goast.Ident:
		return e.Name
	case *goast.SelectorExpr:
		return w.exprToString(e.X) + "." + e.Sel.Name
	case *goast.StarExpr:
		return "*" + w.exprToString(e.X)
	case *goast.ArrayType:
		return "[]" + w.exprToString(e.Elt)
	case *goast.MapType:
		return "map[" + w.exprToString(e.Key) + "]" + w.exprToString(e.Value)
	default:
		return fmt.Sprintf("<未知表达式类型: %T>", expr)
	}
}

func (w *PackageWalker) GetTypeCache() map[string]types.TypeInfo {
	return w.typeCache
}

func (w *PackageWalker) GetImports() map[string]string {
	return w.imports
}

// NewPackageWalker 创建新的遍历器
func NewPackageWalker(graph types.ConversionGraph) *PackageWalker {
	return &PackageWalker{
		graph:          graph,
		imports:        make(map[string]string),
		typeCache:      make(map[string]types.TypeInfo),
		loadedPkgs:     make(map[string]*packages.Package),
		packageMode:    packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		PackageConfigs: make([]*types.PackageConversionConfig, 0),
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

	w.collectPackageImports(pkg)

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

			// Process comments attached to the TypeSpec
			if typeSpec.Doc != nil {
				var typeCfg *types.ConversionConfig
				for _, comment := range typeSpec.Doc.List {
					if strings.HasPrefix(comment.Text, "//go:abgen:") {
						if typeCfg == nil {
							typeCfg = &types.ConversionConfig{
								SourceType:   typeSpec.Name.Name,
								IgnoreFields: make(map[string]bool),
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
func (w *PackageWalker) parseAndApplyDirective(line string, typeCfg *types.ConversionConfig, pkgConfigs map[string]*types.PackageConversionConfig) {
	slog.Debug("parseAndApplyDirective", "line", line)
	directive := strings.TrimPrefix(line, "//go:abgen:")

	parts := strings.SplitN(directive, "=", 2)
	slog.Debug("parseAndApplyDirective", "directive_raw", directive, "parts", parts, "len_parts", len(parts))
	if len(parts) != 2 {
		slog.Debug("parseAndApplyDirective: failed to split directive by =", "directive", directive)
		return
	}
	keyStr := parts[0]
	value := strings.Trim(parts[1], `"`)

	keys := strings.Split(keyStr, ":")
	slog.Debug("parseAndApplyDirective", "keyStr_raw", keyStr, "keys", keys, "len_keys", len(keys))
	if len(keys) < 2 {
		slog.Debug("parseAndApplyDirective: failed to split keyStr by :", "keyStr", keyStr)
		return
	}

	verb := keys[0]
	subject := keys[1]

	slog.Debug("Parsed Directive", "verb", verb, "subject", subject, "keys", keys, "value", value)

	switch {
	case verb == "convert" && subject == "package": // handles directives like //go:abgen:convert:package:source="..."
		if pkgConfigs == nil {
			return
		}
		// Aggregate configs by a constant key since there's usually one package-pair per file.
		const pkgConfigKey = "pkg"
		if pkgConfigs[pkgConfigKey] == nil {
			pkgConfigs[pkgConfigKey] = &types.PackageConversionConfig{
				IgnoreTypes: make(map[string]bool),
				FieldMap:    make(map[string]string),
			}
		}
		cfg := pkgConfigs[pkgConfigKey]

		// The subject is "package", so actual properties are in keys[2] or keys[3]
		if len(keys) == 3 { // e.g., convert:package:source="path"
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
		} else if len(keys) == 4 { // e.g., convert:package:source:suffix="PB"
			entity := keys[2]   // "source" or "target"
			property := keys[3] // "prefix" or "suffix"
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
	case verb == "convert": // handles directives like //go:abgen:convert:target="..."
		if typeCfg == nil || len(keys) < 2 {
			return
		}
		if len(keys) == 2 { // e.g., convert:target="UserPB"
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
		} else if len(keys) == 3 { // e.g., convert:source:suffix="PO"
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

func (w *PackageWalker) collectPackageImports(pkg *packages.Package) {
	for _, file := range pkg.Syntax {
		w.collectImports(file)
	}
}

// cacheTypeAlias 缓存类型别名
func (w *PackageWalker) cacheTypeAlias(name, target string) {
	// 立即将别名添加到缓存，防止循环引用
	w.typeCache[name] = types.TypeInfo{
		Name:       name,
		AliasFor:   target,
		ImportPath: w.currentPkg.PkgPath,
		PkgName:    w.currentPkg.Name, // Set PkgName for aliases
		IsAlias:    true,
		Fields:     []types.StructField{}, // 默认为空
	}

	// 尝试解析目标类型以获取字段信息
	if targetInfo := w.resolveTargetType(target); targetInfo.Name != "" {
		// 更新缓存中的字段信息
		info := w.typeCache[name]
		info.Fields = targetInfo.Fields // 继承目标类型的字段
		w.typeCache[name] = info
		slog.Info("成功解析类型别名的目标类型", "别名", name, "目标", target, "字段数", len(info.Fields))
	}
}

// resolveTargetType 解析目标类型
// resolveTargetType resolves the TypeInfo for a given type name, which may be in an external package.
func (w *PackageWalker) resolveTargetType(targetType string) types.TypeInfo {
	slog.Debug("resolveTargetType: 开始解析目标类型", "targetType", targetType)
	// 1. Check cache first
	if info, exists := w.typeCache[targetType]; exists {
		slog.Debug("resolveTargetType: 从缓存中找到", "targetType", targetType)
		return info
	}

	// 立即将当前类型添加到缓存中，防止循环引用导致死循环
	// 先创建一个占位符，稍后更新实际内容
	w.typeCache[targetType] = types.TypeInfo{Name: targetType}

	// 2. Handle local aliases within all known packages
	slog.Debug("resolveTargetType: 检查所有已知包中的本地别名", "targetType", targetType)
	for _, pkg := range w.allKnownPkgs {
		slog.Debug("resolveTargetType: 检查包", "PkgPath", pkg.PkgPath)
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
					for _, spec := range genDecl.Specs {
						if typeSpec, ok := spec.(*goast.TypeSpec); ok && typeSpec.Name.Name == targetType {
							slog.Debug("resolveTargetType: 在已知包中找到本地别名", "pkg", pkg.PkgPath, "alias", targetType)
							if aliasIdent, aliasOk := typeSpec.Type.(*goast.Ident); aliasOk {
								slog.Debug("resolveTargetType: 别名指向同一包内的类型", "alias", targetType, "target", aliasIdent.Name)
								return w.resolveTargetType(aliasIdent.Name)
							} else if aliasSelector, aliasOk := typeSpec.Type.(*goast.SelectorExpr); aliasOk {
								aliasedTypeName := w.exprToString(aliasSelector)
								slog.Debug("resolveTargetType: 别名指向导入类型", "alias", targetType, "target", aliasedTypeName)
								return w.resolveTargetType(aliasedTypeName)
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
		pkgIdentifier := parts[0] // Could be alias (e.g., "ent") or full PkgPath (e.g., "origadmin/...")
		typeName := parts[1]      // e.g., "User"
		slog.Debug("resolveTargetType: 处理限定类型", "pkgIdentifier", pkgIdentifier, "typeName", typeName)

		var foundPkg *packages.Package

		// First, try to find the package by its full PkgPath if pkgIdentifier is a full path
		slog.Debug("resolveTargetType: 尝试通过完整包路径查找", "pkgIdentifier", pkgIdentifier)
		for _, p := range w.allKnownPkgs {
			slog.Debug("resolveTargetType: 检查已知包", "p.PkgPath", p.PkgPath)
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
					slog.Debug("resolveTargetType: 检查已知包 (通过别名)", "p.PkgPath", p.PkgPath)
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
			slog.Debug("resolveTargetType: foundPkg GoFiles", "GoFiles", foundPkg.GoFiles)
			slog.Debug("resolveTargetType: foundPkg Syntax count", "SyntaxCount", len(foundPkg.Syntax))
			// 5. Find the type spec in the found package
			for _, file := range foundPkg.Syntax {
				slog.Debug("resolveTargetType: 检查文件", "filename", foundPkg.Fset.File(file.Pos()).Name())
				for _, decl := range file.Decls {
					if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
						for _, spec := range genDecl.Specs {
							if typeSpec, ok := spec.(*goast.TypeSpec); ok && typeSpec.Name.Name == typeName {
								slog.Debug("resolveTargetType: 成功找到 TypeSpec", "typeName", typeName)
								// 6. Parse and return its struct info
								info := w.parseStructFields(typeSpec, foundPkg.Name, foundPkg.PkgPath)
								w.typeCache[targetType] = info // Cache the result with full qualifier
								slog.Debug("resolveTargetType: 返回 TypeInfo", "typeName", info.Name, "pkgName", info.PkgName)
								return info
							}
						}
					}
				}
			}
			slog.Debug("resolveTargetType: 未在找到的包的Syntax中找到TypeSpec", "typeName", typeName, "foundPkg.PkgPath", foundPkg.PkgPath)
		} else {
			slog.Debug("resolveTargetType: 未在已知包中找到限定类型所属的包", "pkgIdentifier", pkgIdentifier, "typeName", typeName)
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
										info := w.parseStructFields(typeSpec, foundPkg.Name, foundPkg.PkgPath)
										w.typeCache[targetType] = info
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

	// If the type is not found as a struct, it might be a primitive type or a non-struct type.
	// We still return a TypeInfo with the name.
	slog.Debug("resolveTargetType: 未找到类型，返回基本类型信息", "targetType", targetType)

	// 从完整路径中提取类型名（如果包含路径）
	typeName := targetType
	pkgName := ""
	if strings.Contains(targetType, "/") {
		parts := strings.Split(targetType, "/")
		typeName = parts[len(parts)-1]

		// 尝试从路径中推断包名 - 简化版本
		pathParts := strings.Split(targetType, "/")
		for i, part := range pathParts {
			if part == "services" && i+1 < len(pathParts) {
				pkgName = pathParts[i+1]
				break
			}
		}
		if pkgName == "" && len(pathParts) > 0 {
			// 如果上面没找到，取最后一个目录名
			pkgName = pathParts[len(pathParts)-2]
		}
	}

	slog.Debug("resolveTargetType: 提取的类型信息", "原始", targetType, "类型名", typeName, "包名", pkgName)
	return types.TypeInfo{Name: typeName, PkgName: pkgName, ImportPath: ""}
}

// parseStructFields 解析结构体字段
func (w *PackageWalker) parseStructFields(typeSpec *goast.TypeSpec, pkgName, pkgPath string) types.TypeInfo {
	slog.Debug("parseStructFields: 解析结构体字段", "类型名", typeSpec.Name.Name, "包名", pkgName, "包路径", pkgPath)
	info := types.TypeInfo{
		Name:       typeSpec.Name.Name,
		PkgName:    pkgName, // Set PkgName here
		ImportPath: pkgPath,
		Fields:     []types.StructField{},
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
					embeddedTypeName := w.exprToString(selExpr)
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
					Type:     w.exprToString(field.Type),
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
		typeName := w.exprToString(selector)
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
	default: // "both"
		graphNode.ToConversions = AppendIfNotExists(graphNode.ToConversions, cfg.TargetType)
		w.ensureNodeExists(cfg.TargetType)
		targetNode := w.graph[cfg.TargetType]
		targetNode.FromConversions = AppendIfNotExists(targetNode.FromConversions, cfg.SourceType)
	}
}
