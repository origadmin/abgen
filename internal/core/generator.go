// Package core implements the functions, types, and interfaces for the module.
package core

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/origadmin/abgen/internal/ast"
	"github.com/origadmin/abgen/internal/generator"
	"github.com/origadmin/abgen/internal/template"
	"github.com/origadmin/abgen/internal/types"
	"golang.org/x/tools/go/packages"
)

// ConverterGenerator 代码生成器
type ConverterGenerator struct {
	walker   *ast.PackageWalker
	graph    types.ConversionGraph
	PkgPath  string
	Output   string
	fieldGen *generator.FieldGenerator
	tmplMgr  *template.Manager
}

// NewGenerator 创建新的生成器实例
func NewGenerator() *ConverterGenerator {
	g := &ConverterGenerator{
		graph:   make(types.ConversionGraph),
		tmplMgr: template.NewManager(),
	}
	g.walker = ast.NewPackageWalker(g.graph)
	g.fieldGen = generator.NewFieldGenerator(nil)
	g.fieldGen.SetResolver(g.walker)
	return g
}

// ParseSource 解析目录下的所有Go文件
func (g *ConverterGenerator) ParseSource(dir string) error {
	slog.Info("ParseSource 开始", "目录", dir)
	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return fmt.Errorf("加载包失败: %w", err)
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("未找到有效包: %s", dir)
	}
	g.PkgPath = pkgs[0].PkgPath
	g.walker.AddKnownPackages(pkgs...)
	return g.buildGraph(pkgs)
}

// buildGraph 构建类型转换图
func (g *ConverterGenerator) buildGraph(pkgs []*packages.Package) error {
	for _, pkg := range pkgs {
		if err := g.walker.WalkPackage(pkg); err != nil {
			return fmt.Errorf("遍历包失败: %w", err)
		}
	}

	for _, pkgCfg := range g.walker.PackageConfigs {
		srcPkg, err := g.loadPackage(pkgCfg.SourcePackage)
		if err != nil {
			return fmt.Errorf("无法加载源包 %s: %w", pkgCfg.SourcePackage, err)
		}
		dstPkg, err := g.loadPackage(pkgCfg.TargetPackage)
		if err != nil {
			return fmt.Errorf("无法加载目标包 %s: %w", pkgCfg.TargetPackage, err)
		}
		g.walker.AddKnownPackages(srcPkg, dstPkg)
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
					Source: &types.EndpointConfig{Type: srcTypeName, Prefix: pkgCfg.SourcePrefix, Suffix: pkgCfg.SourceSuffix},
					Target: &types.EndpointConfig{Type: targetTypeName, Prefix: pkgCfg.TargetPrefix, Suffix: pkgCfg.TargetSuffix},
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

func (g *ConverterGenerator) loadPackage(path string) (*packages.Package, error) {
	cfg := &packages.Config{Mode: packages.NeedSyntax | packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo}
	pkgs, err := packages.Load(cfg, path)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no package found at %s", path)
	}
	return pkgs[0], nil
}

// Generate 生成转换代码
func (g *ConverterGenerator) Generate() error {
	slog.Info("开始生成转换代码")
	packageName := filepath.Base(g.PkgPath)
	importMgr := newImportManager(g.PkgPath)
	var funcs []*template.Function
	var typeAliases []string
	generatedFuncs := make(map[string]bool)

	importMgr.GetType("log/slog", "")

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

			if (cfg.Source.Suffix != "" || cfg.Source.Prefix != "") && !importMgr.IsAliasRegistered(srcAlias) {
				typeAliases = append(typeAliases, fmt.Sprintf("type %s = %s", srcAlias, srcLocalType))
				importMgr.RegisterAlias(srcAlias)
			}
			if (cfg.Target.Suffix != "" || cfg.Target.Prefix != "") && !importMgr.IsAliasRegistered(targetAlias) {
				typeAliases = append(typeAliases, fmt.Sprintf("type %s = %s", targetAlias, dstLocalType))
				importMgr.RegisterAlias(targetAlias)
			}
			
			g.fieldGen.SetCustomTypeConversionRules(cfg.TypeConversionRules)
			fields := g.fieldGen.GenerateFields(cfg.Source.Type, cfg.Target.Type, cfg)

			funcs = append(funcs, &template.Function{
				Name:          funcName,
				SourceType:    srcLocalType,
				TargetType:    dstLocalType,
				SourcePointer: "*",
				TargetPointer: "*",
				Conversions:   fields,
			})
		}
	}
	sort.Strings(typeAliases)

	templateData := &template.Data{
		PackageName: packageName,
		Imports:     importMgr.GetImports(),
		TypeAliases: typeAliases,
		Funcs:       funcs,
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

type importManager struct {
	imports           map[string]string
	aliases           map[string]string
	localPackage      string
	registeredAliases map[string]bool
}

func newImportManager(localPackage string) *importManager {
	return &importManager{
		imports:           make(map[string]string),
		aliases:           make(map[string]string),
		localPackage:      localPackage,
		registeredAliases: make(map[string]bool),
	}
}

func (im *importManager) GetType(pkgPath, typeName string) string {
	if pkgPath == "" || pkgPath == im.localPackage {
		return typeName
	}
	alias, exists := im.imports[pkgPath]
	if !exists {
		base := filepath.Base(pkgPath)
		alias = base
		for i := 1; im.aliases[alias] != "" && im.aliases[alias] != pkgPath; i++ {
			alias = fmt.Sprintf("%s%d", base, i)
		}
		im.imports[pkgPath] = alias
		im.aliases[alias] = pkgPath
	}
	return fmt.Sprintf("%s.%s", alias, typeName)
}

func (im *importManager) GetImports() []template.Import {
	var imports []template.Import
	for path, alias := range im.imports {
		imports = append(imports, template.Import{Alias: alias, Path: path})
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i].Path < imports[j].Path })
	return imports
}
func (im *importManager) RegisterAlias(alias string) { im.registeredAliases[alias] = true }
func (im *importManager) IsAliasRegistered(alias string) bool { return im.registeredAliases[alias] }
