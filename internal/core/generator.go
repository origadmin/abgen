// Package core implements the functions, types, and interfaces for the module.
package core

import (
	"fmt"
	goast "go/ast"
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

	// Step 1: Update resolver's walker and add initial packages *before* processing package-level directives
	if resolver, ok := g.resolver.(*ast.TypeResolverImpl); ok {
		resolver.UpdateFromWalker(walker)
		resolver.AddPackages(pkgs...) // Add all initially loaded packages to the resolver's known list
	} else {
		return fmt.Errorf("g.resolver 不是 ast.TypeResolverImpl 类型，无法更新walker或添加包")
	}

	// 处理包级转换
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

		// Step 2: Ensure resolver and walker know about these dynamically loaded packages
		if resolver, ok := g.resolver.(*ast.TypeResolverImpl); ok {
			resolver.AddPackages(srcPkg, dstPkg)
		} else {
			slog.Warn("g.resolver 不是 ast.TypeResolverImpl 类型，无法添加包")
		}
		// THIS IS THE FIX: Update the walker with the newly loaded packages.
		walker.AddKnownPackages(srcPkg, dstPkg)

		// Now, the actual matching loop
		for _, file := range srcPkg.Syntax {
			slog.Debug("buildGraph: 检查源包文件 (匹配循环)", "filename", srcPkg.Fset.File(file.Pos()).Name())
			goast.Inspect(file, func(n goast.Node) bool {
				if ts, ok := n.(*goast.TypeSpec); ok {
					// **FIX**: 检查类型是否为公开的（首字母大写）
					if !ts.Name.IsExported() {
						slog.Debug("buildGraph: 跳过非公开类型", "typeName", ts.Name.Name)
						return true
					}

					// 检查是否为结构体或结构体别名
					if !walker.IsStructOrStructAlias(ts) {
						slog.Debug("buildGraph: 跳过非结构体类型", "typeName", ts.Name.Name)
						return true
					}

					srcTypeName := fmt.Sprintf("%s.%s", srcPkg.PkgPath, ts.Name.Name)
					targetTypeName := fmt.Sprintf("%s.%s", dstPkg.PkgPath, ts.Name.Name)
					slog.Debug("buildGraph: 正在尝试匹配类型", "ts.Name.Name", ts.Name.Name, "srcTypeName", srcTypeName, "targetTypeName", targetTypeName)

					// **FIX**: 确保目标类型真实存在且路径匹配
					targetInfo, err := g.resolver.Resolve(targetTypeName)
					if err == nil && targetInfo.Name == ts.Name.Name && targetInfo.ImportPath == dstPkg.PkgPath {
						slog.Debug("buildGraph: g.resolver.Resolve 成功且包路径匹配", "targetTypeName", targetTypeName)
						if !pkgCfg.IgnoreTypes[ts.Name.Name] {
							slog.Info("发现匹配的包类型", "type", ts.Name.Name, "sourcePkg", srcPkg.PkgPath, "targetPkg", dstPkg.PkgPath)
							convCfg := &types.ConversionConfig{
								SourceType:   srcTypeName,
								TargetType:   targetTypeName,
								Direction:    pkgCfg.Direction,
								IgnoreFields: make(map[string]bool), // TODO: support package-level ignore fields
							}
							walker.AddConversion(convCfg)
						}
					} else {
						slog.Debug("buildGraph: g.resolver.Resolve 失败或目标类型不存在", "targetTypeName", targetTypeName, "error", err)
					}
				}
				return true
			})
		}
	}

	// 输出类型绑定信息以便调试
	fmt.Println("\n======== 类型绑定信息 ========")
	if len(g.graph) == 0 {
		slog.Info("类型转换图为空，未发现任何转换配置。")
	}
	for source, node := range g.graph {
		slog.Info("图节点", "源类型", source)
		if len(node.Configs) == 0 {
			slog.Info("  - 节点中没有转换配置")
		}
		for funcKey, cfg := range node.Configs {
			slog.Info("  - 转换配置",
				"键", funcKey,
				"源类型", cfg.SourceType,
				"目标类型", cfg.TargetType,
				"方向", cfg.Direction,
				"忽略字段数", len(cfg.IgnoreFields))
		}
		slog.Info("  - ToConversions", "目标类型", node.ToConversions)
		slog.Info("  - FromConversions", "源类型", node.FromConversions)
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
		Mode: packages.NeedSyntax | packages.NeedName | packages.NeedFiles |
			packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedTypes | packages.NeedTypesInfo,
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

	// 获取包名和包路径
	packageName := "dto" // 默认包名
	var generatorPkgPath string
	if resolver, ok := g.resolver.(*ast.TypeResolverImpl); ok && len(resolver.Pkgs) > 0 {
		packageName = resolver.Pkgs[0].Name
		generatorPkgPath = resolver.Pkgs[0].PkgPath
		g.PkgPath = generatorPkgPath // Store it in the generator
	}

	data := struct {
		Package    string
		Imports    map[string]string
		Converters []map[string]interface{}
		TypeAliases map[string]types.TypeInfo // **FIX**: 添加类型别名
	}{
		Package:    packageName,
		Imports:    g.resolver.GetImports(),
		TypeAliases: make(map[string]types.TypeInfo),
	}

	// 生成转换器数据
	fmt.Println("\n======== 生成转换配置 ========")
	generatedFuncs := make(map[string]bool) // 用于去重
	for _, node := range g.graph {
		for _, cfg := range node.Configs {
			// **FIX**: Set the generator package path in the config
			cfg.GeneratorPkgPath = generatorPkgPath

			// 解析源和目标类型信息
			sourceInfo, err := g.resolver.Resolve(cfg.SourceType)
			if err != nil {
				slog.Error("无法解析源类型", "type", cfg.SourceType, "error", err)
				continue
			}
			targetInfo, err := g.resolver.Resolve(cfg.TargetType)
			if err != nil {
				slog.Error("无法解析目标类型", "type", cfg.TargetType, "error", err)
				continue
			}

			// **FIX**: 过滤掉未导出的类型
			if !goast.IsExported(sourceInfo.Name) || !goast.IsExported(targetInfo.Name) {
				slog.Debug("跳过未导出类型的转换", "source", sourceInfo.Name, "target", targetInfo.Name)
				continue
			}

			funcName, err := g.buildFuncName(cfg)
			if err != nil {
				slog.Error("创建函数名失败", "source", cfg.SourceType, "target", cfg.TargetType, "error", err)
				continue
			}

			// 检查是否已经生成过相同的函数
			if generatedFuncs[funcName] {
				slog.Debug("跳过重复的函数", "函数名", funcName)
				continue
			}
			generatedFuncs[funcName] = true

			slog.Info("处理函数", "函数名", funcName, "源", cfg.SourceType, "目标", cfg.TargetType)

			fields := g.fieldGen.GenerateFields(cfg.SourceType, cfg.TargetType, cfg)

			// 创建类型别名
			srcAlias := g.createTypeAlias(sourceInfo.Name, sourceInfo.PkgName, cfg.SourcePrefix, cfg.SourceSuffix)
			targetAlias := g.createTypeAlias(targetInfo.Name, targetInfo.PkgName, cfg.TargetPrefix, cfg.TargetSuffix)

			// **FIX**: 收集类型别名, 仅当类型不属于当前包时
			// 添加调试日志
			slog.Debug("收集别名",
				"srcAlias", srcAlias,
				"sourceInfo.Name", sourceInfo.Name,
				"sourceInfo.ImportPath", sourceInfo.ImportPath,
				"generatorPkgPath", generatorPkgPath,
				"targetAlias", targetAlias,
				"targetInfo.Name", targetInfo.Name,
				"targetInfo.ImportPath", targetInfo.ImportPath)

			// 只有当别名指向的类型不在当前生成包内，且别名本身不是当前包的类型名时才添加
			if sourceInfo.ImportPath != generatorPkgPath && sourceInfo.Name != srcAlias {
				slog.Debug("添加源类型别名", "alias", srcAlias, "name", sourceInfo.Name, "importPath", sourceInfo.ImportPath)
				data.TypeAliases[srcAlias] = sourceInfo
			} else {
				slog.Debug("跳过源类型别名 (当前包或别名与类型名相同)", "alias", srcAlias, "name", sourceInfo.Name, "importPath", sourceInfo.ImportPath)
			}
			if targetInfo.ImportPath != generatorPkgPath && targetInfo.Name != targetAlias {
				slog.Debug("添加目标类型别名", "alias", targetAlias, "name", targetInfo.Name, "importPath", targetInfo.ImportPath)
				data.TypeAliases[targetAlias] = targetInfo
			} else {
				slog.Debug("跳过目标类型别名 (当前包或别名与类型名相同)", "alias", targetAlias, "name", targetInfo.Name, "importPath", targetInfo.ImportPath)
			}

			converter := map[string]interface{}{
				"FuncName":    funcName,
				"SourceType":  cfg.SourceType,
				"TargetType":  cfg.TargetType,
				"SourceAlias": srcAlias,
				"TargetAlias": targetAlias,
				"Fields":      fields,
				"SourceInfo":  sourceInfo,
				"TargetInfo":  targetInfo,
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

	// 创建类型别名映射
	srcAlias := g.createTypeAlias(sourceInfo.Name, sourceInfo.PkgName, cfg.SourcePrefix, cfg.SourceSuffix)
	targetAlias := g.createTypeAlias(targetInfo.Name, targetInfo.PkgName, cfg.TargetPrefix, cfg.TargetSuffix)

	return fmt.Sprintf("Convert%sTo%s", srcAlias, targetAlias), nil
}

// createTypeAlias 为类型创建简短的别名
func (g *ConverterGenerator) createTypeAlias(typeName, pkgName, prefix, suffix string) string {
	slog.Debug("createTypeAlias 输入", "原始typeName", typeName, "pkgName", pkgName, "prefix", prefix, "suffix", suffix)

	// 首先从完整路径中提取类型名
	if strings.Contains(typeName, "/") {
		parts := strings.Split(typeName, "/")
		typeName = parts[len(parts)-1]
	}

	// 如果类型名中仍包含包选择器（例如 "types.User"），则将其拆分
	if dotIndex := strings.LastIndex(typeName, "."); dotIndex != -1 {
		// 如果 pkgName 为空，则从类型名中提取它
		if pkgName == "" {
			pkgName = typeName[:dotIndex]
		}
		// 更新 typeName 为不带包选择器的纯类型名
		typeName = typeName[dotIndex+1:]
	}

	// 如果有自定义前缀或后缀，优先使用
	if prefix != "" || suffix != "" {
		result := prefix + typeName + suffix
		slog.Debug("createTypeAlias 输出 (前缀/后缀)", "result", result)
		return result
	}

	// 根据包名创建默认别名
	var result string
	switch pkgName {
	case "ent":
		result = typeName + "Ent"
	case "types":
		// 这是一个基于约定的简单检查
		if strings.HasSuffix(g.PkgPath, "dto") { // 假设在 dto 包中，'types' 通常指向 protobuf
			result = typeName + "PB"
		} else {
			result = typeName + "Types"
		}
	case "typespb": // 直接处理 'typespb' 别名
		result = typeName + "PB"
	case "dto":
		result = typeName + "DTO"
	default:
		if pkgName != "" {
			// 使用包名的首字母大写作为后缀
			result = typeName + strings.Title(pkgName)
		} else {
			// 如果没有包名，则直接使用类型名
			result = typeName
		}
	}

	slog.Debug("createTypeAlias 输出", "result", result)
	return result
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
