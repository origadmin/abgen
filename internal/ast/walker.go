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

			// Process type-specific comments (attached to the GenDecl)
			if genDecl.Doc != nil {
				var typeCfg *types.ConversionConfig
				for _, comment := range genDecl.Doc.List {
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
	directive := strings.TrimPrefix(line, "//go:abgen:")

	parts := strings.SplitN(directive, "=", 2)
	if len(parts) != 2 {
		return
	}
	keyStr := parts[0]
	value := strings.Trim(parts[1], `"`)

	keys := strings.Split(keyStr, ":")
	if len(keys) < 2 {
		return
	}

	verb := keys[0]
	subject := keys[1]

	switch verb {
	case "convert-package":
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

		if len(keys) == 2 { // e.g., convert-package:source="path"
			switch subject {
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
		} else if len(keys) == 3 { // e.g., convert-package:source:suffix="PB"
			property := keys[2]
			switch subject {
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

	case "convert":
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
	info := types.TypeInfo{
		Name:       name,
		AliasFor:   target,
		ImportPath: w.currentPkg.PkgPath,
		PkgName:    w.currentPkg.Name, // Set PkgName for aliases
		IsAlias:    true,
		Fields:     []types.StructField{}, // 默认为空
	}

	// 尝试解析目标类型以获取字段信息
	if targetInfo := w.resolveTargetType(target); targetInfo.Name != "" {
		info.Fields = targetInfo.Fields // 继承目标类型的字段
		slog.Info("成功解析类型别名的目标类型", "别名", name, "目标", target, "字段数", len(info.Fields))
	}

	w.typeCache[name] = info
}

// resolveTargetType 解析目标类型
// resolveTargetType resolves the TypeInfo for a given type name, which may be in an external package.
func (w *PackageWalker) resolveTargetType(targetType string) types.TypeInfo {
	// 1. Check cache first
	if info, exists := w.typeCache[targetType]; exists {
		return info
	}

	// 2. Handle qualified types (e.g., "ent.User")
	parts := strings.Split(targetType, ".")
	if len(parts) > 1 {
		pkgAlias := parts[0]
		typeName := parts[1]

		// 3. Find the full import path from the alias
		if importPath, exists := w.imports[pkgAlias]; exists {
			// 4. Load the external package
			extPkg, err := w.loadPackage(importPath)
			if err != nil {
				slog.Error("无法加载类型别名的目标包", "路径", importPath, "错误", err)
				return types.TypeInfo{}
			}

			// 5. Find the type spec in the external package
			for _, file := range extPkg.Syntax {
				for _, decl := range file.Decls {
					if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
						for _, spec := range genDecl.Specs {
							if typeSpec, ok := spec.(*goast.TypeSpec); ok && typeSpec.Name.Name == typeName {
								// 6. Parse and return its struct info
								info := w.parseStructFields(typeSpec, extPkg.Name, extPkg.PkgPath)
								w.typeCache[targetType] = info // Cache the result with full qualifier
								return info
							}
						}
					}
				}
			}
		}
	}

	// If the type is not found as a struct, it might be a primitive type or a non-struct type.
	// We still return a TypeInfo with the name.
	return types.TypeInfo{Name: targetType, PkgName: w.currentPkg.Name, ImportPath: w.currentPkg.PkgPath}
}

// parseStructFields 解析结构体字段
func (w *PackageWalker) parseStructFields(typeSpec *goast.TypeSpec, pkgName, pkgPath string) types.TypeInfo {
	info := types.TypeInfo{
		Name:       typeSpec.Name.Name,
		PkgName:    pkgName, // Set PkgName here
		ImportPath: pkgPath,
		Fields:     []types.StructField{},
	}

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
	// 初始化节点逻辑
	w.ensureNodeExists(cfg.SourceType)
	w.ensureNodeExists(cfg.TargetType)

	// 配置存储逻辑
	configKey := cfg.SourceType + "2" + cfg.TargetType
	graph := w.graph[cfg.SourceType]
	graph.Configs[configKey] = cfg
	// 方向处理逻辑
	switch cfg.Direction {
	case "to":
		addToConversion(graph, cfg.SourceType, cfg.TargetType)
	case "from":
		addFromConversion(graph, cfg.TargetType, cfg.SourceType)
	default:
		addToConversion(graph, cfg.SourceType, cfg.TargetType)
		addFromConversion(graph, cfg.TargetType, cfg.SourceType)
	}
}

// 通用工具函数
func appendIfNotExists(slice []string, item string) []string {
	for _, v := range slice {
		if v == item {
			return slice
		}
	}
	return append(slice, item)
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
