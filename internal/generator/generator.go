// Package generator implements the functions, types, and interfaces for the module.
package generator

import (
	"fmt"
	goast "go/ast"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/ast"
	"github.com/origadmin/abgen/internal/fieldgen"
	"github.com/origadmin/abgen/internal/template"
	"github.com/origadmin/abgen/internal/types"
)

// ConverterGenerator 代码生成器
type ConverterGenerator struct {
	walker    *ast.PackageWalker
	resolver  ast.TypeResolver // Add this field
	graph     types.ConversionGraph
	PkgPath   string
	Output    string
	fieldGen  *fieldgen.FieldGenerator
	tmplMgr   *template.Manager
	importMgr types.ImportManager // Change type to interface
}

// SetTemplateDir sets the directory for custom type conversion templates in the embedded field generator.
func (g *ConverterGenerator) SetTemplateDir(dir string) {
	if g.fieldGen != nil {
		g.fieldGen.SetTemplateDir(dir)
	}
}

// NewGenerator 创建新的生成器实例
func NewGenerator() *ConverterGenerator {
	g := &ConverterGenerator{
		graph:     make(types.ConversionGraph),
		tmplMgr:   template.NewManager(),
		importMgr: types.NewImportManager(""), // Call types.NewImportManager
	}
	g.walker = ast.NewPackageWalker(g.graph)
	g.resolver = ast.NewResolver(nil) // Create the actual resolver here

	g.fieldGen = fieldgen.New(nil, g.importMgr) // Pass g.importMgr
	g.fieldGen.SetResolver(g.resolver)          // Set the resolver to fieldGen
	return g
}

// ParseSource 解析目录下的所有Go文件
func (g *ConverterGenerator) ParseSource(dir string) error {
	slog.Info("ParseSource 开始", "目录", dir)

	// Phase 1: Load the packages specified by 'dir' (e.g., the directive package itself)
	// We only need basic info and files to scan for directives.
	cfgPhase1 := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedSyntax, // Mode DOES include NeedSyntax
		Dir:  dir,
	}
	directivePkgs, err := packages.Load(cfgPhase1, "./...")
	if err != nil {
		return fmt.Errorf("Phase 1: 加载指令包失败: %w", err)
	}
	if len(directivePkgs) == 0 {
		return fmt.Errorf("Phase 1: 未找到任何有效指令包: %s", dir)
	}

	// Extract all package configs from directives
	var allPackageConfigs []*types.PackageConversionConfig
	for _, pkg := range directivePkgs {
		// Use a temporary walker to extract PackageConfigs from directives
		tempWalker := ast.NewPackageWalker(nil)
		if err := tempWalker.WalkPackage(pkg); err != nil {
			return fmt.Errorf("Phase 1: 遍历指令包失败: %w", err)
		}
		for _, cfg := range tempWalker.PackageConfigs {
			allPackageConfigs = append(allPackageConfigs, cfg)
		}
	}

	if len(allPackageConfigs) == 0 {
		return fmt.Errorf("未找到任何转换指令 (//go:abgen:convert)")
	}

	// Collect all unique source and target package paths from directives
	uniquePkgPaths := make(map[string]bool)
	var explicitLoadPaths []string

	// CRITICAL FIX: Add the output directory's package to the load paths first.
	// This ensures that we analyze the destination package for existing types *before*
	// processing any conversion directives.
	if g.Output != "" {
		// We need to resolve the output directory to a package import path.
		// A simple way is to load it. We'll use a temporary config for this.
		cfgOut := &packages.Config{Mode: packages.NeedName, Dir: g.Output}
		outPkgs, err := packages.Load(cfgOut, ".")
		if err != nil {
			return fmt.Errorf("无法解析输出目录 %s 的包路径: %w", g.Output, err)
		}
		if len(outPkgs) > 0 && outPkgs[0].PkgPath != "" {
			outPkgPath := outPkgs[0].PkgPath
			if !uniquePkgPaths[outPkgPath] {
				slog.Debug("将输出包添加到加载路径", "path", outPkgPath)
				uniquePkgPaths[outPkgPath] = true
				explicitLoadPaths = append(explicitLoadPaths, outPkgPath)
			}
		}
	}

	for _, cfg := range allPackageConfigs {
		if !uniquePkgPaths[cfg.SourcePackage] {
			uniquePkgPaths[cfg.SourcePackage] = true
			explicitLoadPaths = append(explicitLoadPaths, cfg.SourcePackage)
		}
		if !uniquePkgPaths[cfg.TargetPackage] {
			uniquePkgPaths[cfg.TargetPackage] = true
			explicitLoadPaths = append(explicitLoadPaths, cfg.TargetPackage)
		}
	}

	// Add the directive packages themselves to be fully loaded, as their ASTs might be needed for later resolution
	for _, pkg := range directivePkgs {
		if !uniquePkgPaths[pkg.PkgPath] {
			uniquePkgPaths[pkg.PkgPath] = true
			explicitLoadPaths = append(explicitLoadPaths, pkg.PkgPath)
		}
	}

	sort.Strings(explicitLoadPaths) // Sort for deterministic behavior

	// Phase 2: Load all identified packages (source, target, and directive packages) with full syntax and type info
	cfgPhase2 := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo, // Mode DOES include NeedSyntax
		Dir:  dir,                                                                                                                               // Keep original dir for context, though explicitLoadPaths overrides what's loaded
	}
	allLoadedPkgs, err := packages.Load(cfgPhase2, explicitLoadPaths...)
	if err != nil {
		return fmt.Errorf("Phase 2: 加载所有必要包失败: %w", err)
	}
	if len(allLoadedPkgs) == 0 {
		return fmt.Errorf("Phase 2: 未找到任何有效包用于代码生成")
	}

	slog.Debug("ParseSource: Loaded packages (explicit)", "count", len(allLoadedPkgs))
	for i, p := range allLoadedPkgs {
		slog.Debug("ParseSource: Loaded package (explicit)", "index", i, "path", p.PkgPath, "files", len(p.Syntax))
	}

	// Set the main package path for code generation (e.g., for import paths)
	g.PkgPath = directivePkgs[0].PkgPath // The package where directives were found
	g.walker.AddPackages(allLoadedPkgs...)
	g.resolver.AddPackages(allLoadedPkgs...)

	// Fully initialize the importManager for Generator and FieldGenerator now that g.PkgPath is known
	g.importMgr = types.NewImportManager(g.PkgPath) // Call types.NewImportManager
	g.fieldGen.SetImportManager(g.importMgr)        // Update fieldGen's importMgr
	// g.tmplMgr.SetLocalPackage(g.PkgPath)          // Removed - no such method on template.Manager
	// Create a map for quick package lookup
	pkgMap := make(map[string]*packages.Package)
	for _, p := range allLoadedPkgs {
		pkgMap[p.PkgPath] = p
	}

	// Build the graph using ALL package configs collected from directives
	g.walker.PackageConfigs = allPackageConfigs // Overwrite the walker's PackageConfigs with the collected ones
	if err := g.buildGraph(pkgMap); err != nil {
		return err
	}

	if err := g.resolver.UpdateFromWalker(g.walker); err != nil {
		return fmt.Errorf("failed to update resolver from walker: %w", err)
	}

	return nil
}

// buildGraph 构建类型转换图
func (g *ConverterGenerator) buildGraph(pkgMap map[string]*packages.Package) error { // Changed signature
	// First, walk all packages in pkgMap to populate the walker's internal structures
	// (like knownTypes and package aliases, which might be needed for directives)
	for _, pkg := range pkgMap {
		if err := g.walker.WalkPackage(pkg); err != nil {
			return fmt.Errorf("遍历包失败: %w", err)
		}
	}

	for _, pkgCfg := range g.walker.PackageConfigs {
		srcPkg, exists := pkgMap[pkgCfg.SourcePackage] // Retrieve from map
		if !exists {
			return fmt.Errorf("源包 %s 未找到在加载的包中", pkgCfg.SourcePackage)
		}
		dstPkg, exists := pkgMap[pkgCfg.TargetPackage] // Retrieve from map
		if !exists {
			return fmt.Errorf("目标包 %s 未找到在加载的包中", pkgCfg.TargetPackage)
		}
		// g.walker.AddPackages(srcPkg, dstPkg) // Already added in ParseSource
		g.matchTypesInPackages(srcPkg, dstPkg, pkgCfg)
	}
	return nil
}

func (g *ConverterGenerator) matchTypesInPackages(srcPkg, dstPkg *packages.Package, pkgCfg *types.PackageConversionConfig) {
	for _, file := range srcPkg.Syntax {
		goast.Inspect(file, func(n goast.Node) bool {
			ts, ok := n.(*goast.TypeSpec)
			if !ok || !ts.Name.IsExported() || pkgCfg.IgnoreTypes[ts.Name.Name] {
				return true
			}
			if _, ok := ts.Type.(*goast.StructType); !ok {
				return true
			}
			srcTypeName := fmt.Sprintf("%s.%s", srcPkg.PkgPath, ts.Name.Name)
			targetTypeName := fmt.Sprintf("%s.%s", dstPkg.PkgPath, ts.Name.Name)
			if targetInfo, err := g.walker.Resolve(targetTypeName); err == nil && targetInfo.Name == ts.Name.Name {
				slog.Info("发现匹配的包类型", "type", ts.Name.Name)
				g.walker.AddConversion(&types.ConversionConfig{
					Source:              &types.EndpointConfig{Type: srcTypeName, Prefix: pkgCfg.SourcePrefix, Suffix: pkgCfg.SourceSuffix},
					Target:              &types.EndpointConfig{Type: targetTypeName, Prefix: pkgCfg.TargetPrefix, Suffix: pkgCfg.TargetSuffix},
					Direction:           pkgCfg.Direction,
					IgnoreFields:        pkgCfg.IgnoreFields,
					RemapFields:         pkgCfg.RemapFields,
					TypeConversionRules: pkgCfg.TypeConversionRules,
				})
			}
			return true
		})
	}
}

// Generate 生成转换代码
func (g *ConverterGenerator) Generate() error {
	slog.Info("开始生成转换代码")
	packageName := filepath.Base(g.PkgPath)
	importMgr := g.importMgr
	var funcs []*template.Function
	var typeAliases []string
	generatedFuncs := make(map[string]bool)
	seenCustomRuleFuncs := make(map[string]bool)

	importMgr.GetType("log/slog", "")

	// Get all existing type names in the target generation package to check for collisions.
	localTypeNameToFQN := g.resolver.GetLocalTypeNameToFQN()

	for _, node := range g.graph {
		for _, cfg := range node.Configs {
			sourceInfo, _ := g.walker.Resolve(cfg.Source.Type)
			targetInfo, _ := g.walker.Resolve(cfg.Target.Type)

			srcAlias := cfg.Source.Prefix + sourceInfo.Name + cfg.Source.Suffix
			targetAlias := cfg.Target.Prefix + targetInfo.Name + cfg.Target.Suffix
			funcName := fmt.Sprintf("Convert%sTo%s", srcAlias, targetAlias)

			if generatedFuncs[funcName] {
				continue
			}
			generatedFuncs[funcName] = true

			srcLocalType := importMgr.GetType(sourceInfo.ImportPath, sourceInfo.Name)
			dstLocalType := importMgr.GetType(targetInfo.ImportPath, targetInfo.Name)

			// Check if the alias name *itself* already exists as a defined type in the target package.
			if _, exists := localTypeNameToFQN[srcAlias]; !exists {
				typeAliases = append(typeAliases, fmt.Sprintf("type %s = %s", srcAlias, srcLocalType))
				importMgr.RegisterAlias(srcAlias) // Mark as "to be generated"
			} else {
				slog.Debug("Generate: skipping alias generation, name already exists in target package", "alias", srcAlias)
			}

			if _, exists := localTypeNameToFQN[targetAlias]; !exists {
				typeAliases = append(typeAliases, fmt.Sprintf("type %s = %s", targetAlias, dstLocalType))
				importMgr.RegisterAlias(targetAlias) // Mark as "to be generated"
			} else {
				slog.Debug("Generate: skipping alias generation, name already exists in target package", "alias", targetAlias)
			}

			g.fieldGen.SetCustomTypeConversionRules(cfg.TypeConversionRules)

			for _, rule := range cfg.TypeConversionRules {
				if rule.ConvertFunc != "" {
					seenCustomRuleFuncs[rule.ConvertFunc] = true
				}
			}
			fields := g.fieldGen.GenerateFields(cfg.Source.Type, cfg.Target.Type, cfg)

			funcs = append(funcs, &template.Function{
				Name:          funcName,
				SourceType:    srcAlias,
				TargetType:    targetAlias,
				SourcePointer: "*",
				TargetPointer: "*",
				Conversions:   fields,
			})
		}
	}
	sort.Strings(typeAliases)

	var customRuleFuncs []string
	for funcName := range seenCustomRuleFuncs {
		customRuleFuncs = append(customRuleFuncs, funcName)
	}
	sort.Strings(customRuleFuncs)

	templateData := &template.Data{
		PackageName:     packageName,
		Imports:         importMgr.GetImports(),
		TypeAliases:     typeAliases,
		Funcs:           funcs,
		CustomRuleFuncs: customRuleFuncs,
	}

	output, err := g.tmplMgr.Render("generator.tpl", templateData)
	if err != nil {
		return fmt.Errorf("渲染模板失败: %w", err)
	}
	outFile := filepath.Join(g.Output, fmt.Sprintf("%s.gen.go", packageName))
	if err := os.WriteFile(outFile, output, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	slog.Info("生成完成", "文件", outFile)
	return nil
}
