// Package ast implements the functions, types, and interfaces for the module.
package ast

import (
	"fmt"
	"go/ast"
	"go/token"
	"log/slog"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/types"
)

// PackageWalker 包遍历器
type PackageWalker struct {
	imports    map[string]string
	graph      types.ConversionGraph
	currentPkg *packages.Package
	cacheTypes map[string]types.TypeInfo
}

// NewPackageWalker 创建新的遍历器
func NewPackageWalker(graph types.ConversionGraph) *PackageWalker {
	return &PackageWalker{
		graph:      graph,
		imports:    make(map[string]string),
		cacheTypes: make(map[string]types.TypeInfo),
	}
}

// WalkPackage 遍历包内的类型定义
// WalkPackage 整合后的包遍历逻辑
func (w *PackageWalker) WalkPackage(pkg *packages.Package) error {
	slog.Info("开始遍历包", "包", pkg.PkgPath)
	w.currentPkg = pkg

	// 统一收集导入信息
	w.collectPackageImports(pkg)

	// 统一处理类型声明
	for _, file := range pkg.Syntax {
		slog.Info("遍历目录", "文件", file.Name)
		if pkg.Fset == nil {
			continue
		}

		offset := pkg.Fset.File(file.Pos())
		if offset == nil {
			continue
		}
		filename := offset.Name()
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

// processFileDecls 统一处理文件声明
func (w *PackageWalker) processFileDecls(file *ast.File) error {
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			// 处理类型声明
			if err := w.processTypeDecl(genDecl); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *PackageWalker) processTypeDecl(genDecl *ast.GenDecl) error {
	for _, spec := range genDecl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		// 处理类型别名逻辑
		if typeSpec.Assign != 0 {
			targetType := w.exprToString(typeSpec.Type)

			// 有转换需求才输出日志
			if cfg := w.parseTypeComments(typeSpec); cfg != nil {
				slog.Info("处理类型别名", "类型名", typeSpec.Name.Name, "目标类型", targetType)
				slog.Info("处理类型文档", "目标信息", cfg)
				w.addConversion(cfg)
			}
			w.cacheTypeAlias(typeSpec.Name.Name, targetType)
		}

		// 处理转换配置
		if cfg := w.parseTypeComments(typeSpec); cfg != nil {
			w.addConversion(cfg)
		}
	}
	return nil
}

func (w *PackageWalker) collectPackageImports(pkg *packages.Package) {
	for _, file := range pkg.Syntax {
		w.collectImports(file)
	}
}

// cacheTypeAlias 缓存类型别名
func (w *PackageWalker) cacheTypeAlias(name, target string) {
	w.cacheTypes[name] = types.TypeInfo{
		Name:       name,
		AliasFor:   target,
		ImportPath: w.currentPkg.PkgPath,
	}
}

// collectImports 收集导入信息
func (w *PackageWalker) collectImports(file *ast.File) {
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
func (w *PackageWalker) addConversion(cfg *types.ConversionConfig) {
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

// walkGenDecl 处理通用声明
func (w *PackageWalker) walkGenDecl(decl *ast.GenDecl) error {
	if decl.Tok != token.TYPE {
		return nil
	}

	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		// 解析类型注释
		cfg := w.parseTypeComments(typeSpec)
		if cfg != nil {
			w.addConversion(cfg)
		}
	}
	return nil
}

// parseTypeComments 解析类型注释
func (w *PackageWalker) parseTypeComments(typeSpec *ast.TypeSpec) *types.ConversionConfig {
	if typeSpec.Doc == nil {
		return nil
	}

	fullComment := ""
	for _, comment := range typeSpec.Doc.List {
		fullComment += strings.TrimSpace(strings.TrimPrefix(comment.Text, "//")) + "\n"
	}

	if !strings.Contains(fullComment, "@Convert") {
		return nil
	}

	return parseConvertComment(fullComment, typeSpec.Name.Name)
}

// parseConvertComment 解析转换注释
func parseConvertComment(comment, typeName string) *types.ConversionConfig {
	params := parseCommentParams(comment)

	cfg := &types.ConversionConfig{
		SourceType:   typeName,
		TargetType:   strings.Trim(params["target"], "\""),
		Direction:    strings.Trim(params["direction"], "\""),
		IgnoreFields: make(map[string]bool),
	}

	if fields, ok := params["ignoreFields"]; ok {
		for _, f := range strings.Split(fields, ",") {
			cfg.IgnoreFields[strings.TrimSpace(f)] = true
		}
	}

	if cfg.Direction == "" {
		cfg.Direction = "both"
	}

	return cfg
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
func (w *PackageWalker) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return w.exprToString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + w.exprToString(e.X)
	case *ast.ArrayType:
		return "[]" + w.exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + w.exprToString(e.Key) + "]" + w.exprToString(e.Value)
	default:
		return fmt.Sprintf("<未知表达式类型: %T>", expr)
	}
}
