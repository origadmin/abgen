// Package core implements the functions, types, and interfaces for the module.
package core

import (
	"fmt"
	goast "go/ast"
	"log/slog"
	"os"
	"path/filepath"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/ast"
	"github.com/origadmin/abgen/internal/generator"
	"github.com/origadmin/abgen/internal/template"
	"github.com/origadmin/abgen/internal/types"
)

// ConverterGenerator 代码生成器
type ConverterGenerator struct {
	resolver ast.TypeResolver
	graph    types.ConversionGraph
	//parsedPkgs []*packages.Package
	PkgPath  string
	Output   string
	fieldGen *generator.FieldGenerator
	tmplMgr  template.Renderer
	tmplConv *template.Convert
}

// ParseSource 解析目录下的所有Go文件
func (g *ConverterGenerator) ParseSource(dir string, convTemp string) error {
	slog.Info("ParseSource 开始", "目录", dir, "模板", convTemp)
	
	if convTemp != "" {
		err := g.tmplConv.LoadExternalTemplates(convTemp)
		if err != nil {
			return fmt.Errorf("加载转换模板失败: %w", err)
		}
	}
	fileInfo, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("路径不存在: %w", err)
	}
	slog.Info("开始加载包", "目录", dir, "是否目录", fileInfo.IsDir())
	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedName | packages.NeedFiles |
			packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedTypes | packages.NeedTypesInfo,
		Dir: dir,
	}

	filename := "./..."
	if !fileInfo.IsDir() {
		cfg.Dir = filepath.Dir(dir)
		filename = filepath.Base(dir)
		slog.Info("开始加载文件", "文件", filename)
	} else {
		slog.Info("开始加载目录", "路径", dir)
	}

	pkgs, err := packages.Load(cfg, filename)
	if err != nil {
		return fmt.Errorf("加载包失败: %w", err)
	}

	slog.Info("包加载结果", "包数量", len(pkgs), "错误", err)
	if len(pkgs) == 0 {
		return fmt.Errorf("未找到有效包，检查路径: %s", dir)
	}

	for _, pkg := range pkgs {
		slog.Info("成功加载包",
			"ID", pkg.ID,
			"Name", pkg.Name,
			"PkgPath", pkg.PkgPath,
			"GoFiles", len(pkg.GoFiles),
			"Syntax", len(pkg.Syntax))
	}

	// 将包信息传递给resolver
	if resolver, ok := g.resolver.(*ast.TypeResolverImpl); ok {
		resolver.Pkgs = pkgs
		slog.Info("更新resolver包信息", "包数量", len(pkgs))
	}
	
	return g.buildGraph(pkgs)
}

// buildGraph 构建类型转换图
func (g *ConverterGenerator) buildGraph(pkgs []*packages.Package) error {
	fmt.Println("======== 开始构建类型转换图 ========")
	walker := ast.NewPackageWalker(g.graph)
	for _, pkg := range pkgs {
		slog.Info("开始遍历包", "包", pkg.PkgPath, "文件数", len(pkg.GoFiles))
		if err := walker.WalkPackage(pkg); err != nil {
			return fmt.Errorf("遍历包失败: %w", err)
		}
	}

	// Handle package-level conversions
	for _, pkgCfg := range walker.PackageConfigs {
		slog.Info("处理包级转换配置", "source", pkgCfg.SourcePackage, "target", pkgCfg.TargetPackage)

		srcPkg, err := g.loadPackage(pkgCfg.SourcePackage)
		if err != nil {
			return fmt.Errorf("无法加载源包 %s: %w", pkgCfg.SourcePackage, err)
		}
		dstPkg, err := g.loadPackage(pkgCfg.TargetPackage)
		if err != nil {
			return fmt.Errorf("无法加载目标包 %s: %w", pkgCfg.TargetPackage, err)
		}

		// Collect all type names from destination package for quick lookup
		dstTypes := make(map[string]bool)
		for _, file := range dstPkg.Syntax {
			goast.Inspect(file, func(n goast.Node) bool {
				if ts, ok := n.(*goast.TypeSpec); ok {
					dstTypes[ts.Name.Name] = true
				}
				return true
			})
		}

		// Iterate source package types and find matches
		for _, file := range srcPkg.Syntax {
			goast.Inspect(file, func(n goast.Node) bool {
				if ts, ok := n.(*goast.TypeSpec); ok {
					if _, exists := dstTypes[ts.Name.Name]; exists && !pkgCfg.IgnoreTypes[ts.Name.Name] {
						slog.Info("发现匹配的包类型", "type", ts.Name.Name)
						convCfg := &types.ConversionConfig{
							SourceType:   fmt.Sprintf("%s.%s", srcPkg.Name, ts.Name.Name),
							TargetType:   fmt.Sprintf("%s.%s", dstPkg.Name, ts.Name.Name),
							Direction:    pkgCfg.Direction,
							IgnoreFields: make(map[string]bool), // TODO: support package-level ignore fields
						}
						walker.AddConversion(convCfg)
					}
				}
				return true
			})
		}
	}

	// 输出类型绑定信息以便调试
	fmt.Println("\n======== 类型绑定信息 ========")
	for source, node := range g.graph {
		slog.Info("遍历类型", "源", source)
		for funcName, cfg := range node.Configs {
			slog.Info("遍历函数", "函数名", funcName)
			slog.Info("遍历转换", "源", cfg.SourceType, "目标", cfg.TargetType)
		}
	}

	// 将walker收集的导入信息传递给resolver
	if err := g.resolver.UpdateFromWalker(walker); err != nil {
		return fmt.Errorf("更新类型解析器失败: %w", err)
	}

	// 共享相同的resolver给字段生成器
	g.fieldGen.SetResolver(g.resolver)

	// 更新FieldGenerator的TypeMap
	g.fieldGen.UpdateTypeMap()

	// 输出已知类型列表
	fmt.Println("\n======== 已注册的类型 ========")
	knownTypes := g.resolver.GetKnownTypes()
	for typeName := range knownTypes {
		slog.Info("遍历类型", "类型", typeName)
	}

	return nil
}

func (g *ConverterGenerator) loadPackage(path string) (*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedSyntax,
	}
	pkgs, err := packages.Load(cfg, path)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no package found at %s", path)
	}
	if pkgs[0].Errors != nil && len(pkgs[0].Errors) > 0 {
		return nil, fmt.Errorf("errors loading package %s: %v", path, pkgs[0].Errors)
	}
	return pkgs[0], nil
}

// Generate 生成转换代码
func (g *ConverterGenerator) Generate() error {
	fmt.Println("\n======== 开始生成转换代码 ========")

	// 获取包名
	packageName := "dto" // 默认包名
	if resolver, ok := g.resolver.(*ast.TypeResolverImpl); ok && len(resolver.Pkgs) > 0 {
		packageName = resolver.Pkgs[0].Name
	}

	data := struct {
		Package    string
		Imports    map[string]string
		Converters []map[string]interface{}
	}{
		Package: packageName,
		Imports: g.resolver.GetImports(),
	}

	// 生成转换器数据
	fmt.Println("\n======== 生成转换配置 ========")
	for _, node := range g.graph {
		for _, cfg := range node.Configs {
			funcName, err := g.buildFuncName(cfg)
			if err != nil {
				slog.Error("创建函数名失败", "source", cfg.SourceType, "target", cfg.TargetType, "error", err)
				continue
			}

			slog.Info("处理函数", "函数名", funcName, "源", cfg.SourceType, "目标", cfg.TargetType)
			fields := g.fieldGen.GenerateFields(cfg.SourceType, cfg.TargetType, cfg)

			converter := map[string]interface{}{
				"FuncName":   funcName,
				"SourceType": cfg.SourceType,
				"TargetType": cfg.TargetType,
				"Fields":     fields,
			}
			data.Converters = append(data.Converters, converter)
		}
	}

	// 使用模板管理器渲染
	output, err := g.tmplMgr.Render("", data)
	if err != nil {
		return fmt.Errorf("渲染模板失败: %w", err)
	}

	// 写入生成的代码
	outFile := filepath.Join(g.Output, fmt.Sprintf("%s.gen.go", packageName))
	if err := os.WriteFile(outFile, output, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	slog.Info("生成完成", "文件", outFile)
	return nil
}

func (g *ConverterGenerator) buildFuncName(cfg *types.ConversionConfig) (string, error) {
	sourceInfo, err := g.resolver.Resolve(cfg.SourceType)
	if err != nil {
		return "", fmt.Errorf("无法解析源类型 %s: %w", cfg.SourceType, err)
	}
	targetInfo, err := g.resolver.Resolve(cfg.TargetType)
	if err != nil {
		return "", fmt.Errorf("无法解析目标类型 %s: %w", cfg.TargetType, err)
	}

	caser := cases.Title(language.Und, cases.NoLower)

	// Build source name part
	srcNamePart := sourceInfo.Name
	if cfg.SourcePrefix != "" || cfg.SourceSuffix != "" {
		srcNamePart = cfg.SourcePrefix + srcNamePart + cfg.SourceSuffix
	} else if sourceInfo.PkgName != "" { // Default to package name as prefix if no custom affix
		srcNamePart = caser.String(sourceInfo.PkgName) + srcNamePart
	}

	// Build target name part
	targetNamePart := targetInfo.Name
	if cfg.TargetPrefix != "" || cfg.TargetSuffix != "" {
		targetNamePart = cfg.TargetPrefix + targetNamePart + cfg.TargetSuffix
	} else if targetInfo.PkgName != "" { // Default to package name as prefix if no custom affix
		targetNamePart = caser.String(targetInfo.PkgName) + targetNamePart
	}

	return fmt.Sprintf("Convert%sTo%s", srcNamePart, targetNamePart), nil
}

// NewGenerator 创建新的生成器实例
func NewGenerator() *ConverterGenerator {
	return &ConverterGenerator{
		graph:    make(types.ConversionGraph),
		resolver: ast.NewResolver(nil), // 初始化时没有包信息
		tmplMgr:  template.NewManager(),
		tmplConv: template.NewConverter(),
		fieldGen: generator.NewFieldGenerator(),
	}
}

// SetTemplateDir 设置模板目录
func (g *ConverterGenerator) SetTemplateDir(dir string) {
	// 设置字段生成器的模板目录
	g.fieldGen.SetTemplateDir(dir)

	// 同时让模板管理器加载目录中的模板
	if manager, ok := g.tmplMgr.(*template.Generator); ok {
		if err := manager.LoadFromDir(dir); err != nil {
			slog.Error("加载模板失败", "错误", err)
		}
	}

	// 加载完模板后更新类型映射表
	if err := g.fieldGen.LoadTemplatesFromDir(); err != nil {
		slog.Error("加载模板失败", "错误", err)
	}
}
