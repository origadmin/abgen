// Package core implements the functions, types, and interfaces for the module.
package core

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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
	slog.Info("开始加载包", "目录", dir)
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

	if len(pkgs) == 0 {
		return fmt.Errorf("未找到有效包，检查路径: %s", dir)
	}

	for _, pkg := range pkgs {
		slog.Info("成功加载包",
			"ID", pkg.ID,
			"Name", pkg.Name,
			"GoFiles", len(pkg.GoFiles))
	}

	return g.buildGraph(pkgs)
}

// buildGraph 构建类型转换图
func (g *ConverterGenerator) buildGraph(pkgs []*packages.Package) error {
	fmt.Println("======== 开始构建类型转换图 ========")
	walker := ast.NewPackageWalker(g.graph)
	for _, pkg := range pkgs {
		slog.Info("开始遍历包", "包", pkg.PkgPath)
		if err := walker.WalkPackage(pkg); err != nil {
			return fmt.Errorf("遍历包失败: %w", err)
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

// Generate 生成转换代码
func (g *ConverterGenerator) Generate() error {
	fmt.Println("\n======== 开始生成转换代码 ========")
	//if len(g.parsedPkgs) == 0 {
	//	return fmt.Errorf("no packages parsed")
	//}
	//
	//packageName := g.parsedPkgs[0].Name

	data := struct {
		Package    string
		Imports    map[string]string
		Converters []map[string]interface{}
	}{
		Package: "TODO",
		Imports: g.resolver.GetImports(),
	}

	// 生成转换器数据
	fmt.Println("\n======== 生成转换配置 ========")
	for _, node := range g.graph {
		for funcName, cfg := range node.Configs {
			slog.Info("处理函数", "函数名", funcName, "源", cfg.SourceType, "目标", cfg.TargetType)
			fields := g.fieldGen.GenerateFields(cfg.SourceType, cfg.TargetType, cfg)
			slog.Info("生成字段", "字段数", len(fields))
			for _, field := range fields {
				slog.Info("处理字段", "字段名", field)
			}

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

	// 确保输出目录存在
	if err := os.MkdirAll(filepath.Dir(g.Output), 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 获取输出文件名
	outputFile := ""
	// 如果输入是目录，则使用包名
	if stat, err := os.Stat(g.Output); err == nil && stat.IsDir() {
		outputFile = "TODO"
	} else {
		// 否则使用输入文件的基础名称
		outputFile = filepath.Base(g.Output)
		g.Output = filepath.Dir(g.Output)
		outputFile = strings.TrimSuffix(outputFile, filepath.Ext(outputFile))
	}

	// 写入生成的代码
	outFile := filepath.Join(g.Output, fmt.Sprintf("%s.gen.go", outputFile))
	if err := os.WriteFile(outFile, output, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	slog.Info("生成完成", "文件", outFile)
	return nil
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
