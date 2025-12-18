package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"log/slog"
	"strings"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/generator/components"
	"github.com/origadmin/abgen/internal/model"
)

// LegacyCodeGenerator 协调所有生成器组件的主生成器
type LegacyCodeGenerator struct {
	config           *config.Config
	importManager    model.ImportManager
	nameGenerator    model.NameGenerator
	aliasManager     model.AliasManager
	conversionEngine model.ConversionEngine
	codeEmitter      model.CodeEmitter
	typeConverter    model.TypeConverter
}

// NewLegacyCodeGenerator 创建新的协调器
func NewLegacyCodeGenerator(config *config.Config, typeInfos map[string]*model.TypeInfo) model.CodeGenerator {
	// 创建导入管理器
	importManager := components.NewImportManager()

	// 创建类型转换器
	typeConverter := components.NewTypeConverter()

	// 创建别名映射
	aliasMap := make(map[string]string)

	// 创建命名生成器
	nameGenerator := components.NewNameGenerator(config, aliasMap)

	// 创建别名管理器
	aliasManager := components.NewAliasManager(config, nameGenerator, typeInfos)

	// 创建转换引擎
	conversionEngine := components.NewConversionEngine(
		typeConverter,
		nameGenerator,
		aliasManager,
		importManager,
	)

	// 创建代码发射器
	codeEmitter := components.NewCodeEmitter(
		config,
		importManager,
		aliasManager,
	)

	return &LegacyCodeGenerator{
		config:           config,
		importManager:    importManager,
		nameGenerator:    nameGenerator,
		aliasManager:     aliasManager,
		conversionEngine: conversionEngine,
		codeEmitter:      codeEmitter,
		typeConverter:    typeConverter,
	}
}

// Generate 实现 model.CodeGenerator 接口
func (g *LegacyCodeGenerator) Generate(request *model.GenerationRequest) (*model.GenerationResponse, error) {
	typeInfos := request.Context.TypeInfos
	g.config = request.Context.Config

	slog.Debug("Generating code with orchestrator",
		"type_count", len(typeInfos),
		"initial_rules", len(g.config.ConversionRules))

	// 填充涉及的包集合
	for _, pkgPath := range g.config.RequiredPackages() {
		request.Context.InvolvedPackages[pkgPath] = struct{}{}
	}

	// 如果没有显式定义转换规则，则从包对中发现它们
	// 这对于启用 `pair:packages` 工作至关重要
	if len(g.config.ConversionRules) == 0 {
		g.discoverImplicitConversionRules(typeInfos)
		slog.Debug("Implicit rule discovery finished", "discovered_rules", len(g.config.ConversionRules))
	}

	// 现在所有规则都已发现，填充命名器的源包映射
	g.nameGenerator.PopulateSourcePkgs(g.config)

	// 智能默认后缀：仅在没有显式命名规则设置时应用
	// 并且如果至少有一个转换规则具有模糊（相同）的基本名称
	if g.config.NamingRules.SourcePrefix == "" && g.config.NamingRules.SourceSuffix == "" &&
		g.config.NamingRules.TargetPrefix == "" && g.config.NamingRules.TargetSuffix == "" {

		needsDisambiguation := false
		for _, rule := range g.config.ConversionRules {
			sourceBaseName := ""
			if lastDot := strings.LastIndex(rule.SourceType, "."); lastDot != -1 {
				sourceBaseName = rule.SourceType[lastDot+1:]
			}

			targetBaseName := ""
			if lastDot := strings.LastIndex(rule.TargetType, "."); lastDot != -1 {
				targetBaseName = rule.TargetType[lastDot+1:]
			}

			if sourceBaseName != "" && sourceBaseName == targetBaseName {
				needsDisambiguation = true
				break
			}
		}

		if needsDisambiguation {
			slog.Debug("Ambiguous type names found with no explicit naming rules. Applying default 'Source'/'Target' suffixes.")
			g.config.NamingRules.SourceSuffix = "Source"
			g.config.NamingRules.TargetSuffix = "Target"
		}
	}

	// 填充别名
	if am, ok := g.aliasManager.(interface{ PopulateAliases() }); ok {
		am.PopulateAliases()
	} else {
		slog.Warn("AliasManager does not implement PopulateAliases method. Skipping alias population.")
	}

	// 生成代码
	generatedCode, err := g.generateMainCode(typeInfos)
	if err != nil {
		return nil, fmt.Errorf("failed to generate main code: %w", err)
	}

	// 生成自定义存根
	customStubs, err := g.generateCustomStubs()
	if err != nil {
		return nil, fmt.Errorf("failed to generate custom stubs: %w", err)
	}

	// 获取所需的包列表
	requiredPackages := g.importManager.GetAllImports()

	return &model.GenerationResponse{
		GeneratedCode:    generatedCode,
		CustomStubs:      customStubs,
		RequiredPackages: requiredPackages,
	}, nil
}

// generateMainCode 生成主要代码
func (g *LegacyCodeGenerator) generateMainCode(typeInfos map[string]*model.TypeInfo) ([]byte, error) {
	finalBuf := new(bytes.Buffer)

	// 发射头信息
	err := g.codeEmitter.EmitHeader(finalBuf)
	if err != nil {
		return nil, err
	}

	// 发射导入语句
	err = g.codeEmitter.EmitImports(finalBuf, g.importManager.GetAllImports())
	if err != nil {
		return nil, err
	}

	// 发射类型别名
	err = g.codeEmitter.EmitAliases(finalBuf)
	if err != nil {
		return nil, err
	}

	// 发射转换函数
	err = g.codeEmitter.EmitConversions(finalBuf, nil) // Placeholder for conversions
	if err != nil {
		return nil, err
	}

	// 发射辅助函数
	err = g.codeEmitter.EmitHelpers(finalBuf, nil) // Placeholder for helpers
	if err != nil {
		return nil, err
	}

	// 格式化代码
	return format.Source(finalBuf.Bytes())
}

// generateCustomStubs 生成自定义存根
func (g *LegacyCodeGenerator) generateCustomStubs() ([]byte, error) {
	customStubs := make(map[string]string)

	// 如果有自定义存根，生成它们
	if len(customStubs) == 0 {
		return nil, nil
	}

	buf := new(bytes.Buffer)
	buf.WriteString("//go:build !abgen_source\n")
	buf.WriteString("// Code generated by abgen. DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("// versions: %s\n", "abgen"))
	buf.WriteString(fmt.Sprintf("// source: %s\n\n", g.config.GenerationContext.DirectivePath))
	buf.WriteString(fmt.Sprintf("package %s\n\n", g.getPackageName()))

	err := g.codeEmitter.EmitImports(buf, g.importManager.GetAllImports())
	if err != nil {
		return nil, err
	}

	err = g.writeCustomStubsToBuffer(buf, customStubs)
	if err != nil {
		return nil, err
	}

	return format.Source(buf.Bytes())
}

// writeCustomStubsToBuffer 将自定义存根写入缓冲区
func (g *LegacyCodeGenerator) writeCustomStubsToBuffer(buf *bytes.Buffer, customStubs map[string]string) error {
	if len(customStubs) == 0 {
		return nil
	}

	buf.WriteString("\n// --- Custom Conversion Stubs ---\n")
	stubNames := make([]string, 0, len(customStubs))
	for name := range customStubs {
		stubNames = append(stubNames, name)
	}

	for _, name := range stubNames {
		buf.WriteString(customStubs[name])
		buf.WriteString("\n")
	}

	return nil
}

// discoverImplicitConversionRules 发现隐式转换规则
func (g *LegacyCodeGenerator) discoverImplicitConversionRules(typeInfos map[string]*model.TypeInfo) {
	typesByPackage := make(map[string][]*model.TypeInfo)
	for _, info := range typeInfos {
		if info.IsNamedType() {
			typesByPackage[info.ImportPath] = append(typesByPackage[info.ImportPath], info)
		}
	}

	var initialRules []*config.ConversionRule

	for _, pair := range g.config.PackagePairs {
		sourceTypes := typesByPackage[pair.SourcePath]
		targetTypes := typesByPackage[pair.TargetPath]

		targetMap := make(map[string]*model.TypeInfo)
		for _, tt := range targetTypes {
			targetMap[tt.Name] = tt
		}

		for _, sourceType := range sourceTypes {
			if targetType, ok := targetMap[sourceType.Name]; ok {
				rule := &config.ConversionRule{
					SourceType: sourceType.UniqueKey(),
					TargetType: targetType.UniqueKey(),
					Direction:  config.DirectionBoth,
					FieldRules: config.FieldRuleSet{Ignore: make(map[string]struct{}), Remap: make(map[string]string)},
				}
				initialRules = append(initialRules, rule)
			}
		}
	}

	g.config.ConversionRules = append(g.config.ConversionRules, initialRules...)
}

// getPackageName 获取包名
func (g *LegacyCodeGenerator) getPackageName() string {
	if g.config.GenerationContext.PackageName != "" {
		return g.config.GenerationContext.PackageName
	}
	return "generated"
}

// GetConfig 获取配置（用于向后兼容）
func (g *LegacyCodeGenerator) GetConfig() *config.Config {
	return g.config
}
