package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"log/slog"
	"sort"
	"strings"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/generator/components"
	"github.com/origadmin/abgen/internal/model"
)

// CodeGenerator is the main generator that orchestrates all generator components.
type CodeGenerator struct {
	config           *config.Config
	importManager    model.ImportManager
	nameGenerator    model.NameGenerator
	aliasManager     model.AliasManager
	conversionEngine model.ConversionEngine
	codeEmitter      model.CodeEmitter
	typeConverter    model.TypeConverter
	typeInfos        map[string]*model.TypeInfo
}

// NewCodeGenerator creates a new orchestrator.
func NewCodeGenerator(config *config.Config, typeInfos map[string]*model.TypeInfo) model.CodeGenerator {
	importManager := components.NewImportManager()
	typeConverter := components.NewTypeConverter()
	aliasMap := make(map[string]string)
	nameGenerator := components.NewNameGenerator(config, aliasMap)
	// 修复：传递importManager给AliasManager
	aliasManager := components.NewAliasManager(config, nameGenerator, importManager, typeInfos)
	conversionEngine := components.NewConversionEngine(
		typeConverter,
		nameGenerator,
		aliasManager,
		importManager,
	)
	codeEmitter := components.NewCodeEmitter(
		config,
		importManager,
		aliasManager,
	)

	return &CodeGenerator{
		config:           config,
		importManager:    importManager,
		nameGenerator:    nameGenerator,
		aliasManager:     aliasManager,
		conversionEngine: conversionEngine,
		codeEmitter:      codeEmitter,
		typeConverter:    typeConverter,
		typeInfos:        typeInfos,
	}
}

// Generate implements the model.CodeGenerator interface.
func (g *CodeGenerator) Generate(request *model.GenerationRequest) (*model.GenerationResponse, error) {
	g.typeInfos = request.Context.TypeInfos
	g.config = request.Context.Config

	slog.Debug("Generating code with orchestrator",
		"type_count", len(g.typeInfos),
		"initial_rules", len(g.config.ConversionRules))

	for _, pkgPath := range g.config.RequiredPackages() {
		request.Context.InvolvedPackages[pkgPath] = struct{}{}
	}

	if len(g.config.ConversionRules) == 0 {
		g.discoverImplicitConversionRules(g.typeInfos)
		slog.Debug("Implicit rule discovery finished", "discovered_rules", len(g.config.ConversionRules))
	}

	g.nameGenerator.PopulateSourcePkgs(g.config)

	if g.config.NamingRules.SourcePrefix == "" && g.config.NamingRules.SourceSuffix == "" &&
		g.config.NamingRules.TargetPrefix == "" && g.config.NamingRules.TargetSuffix == "" {
		if g.needsDisambiguation() {
			slog.Debug("Ambiguous type names found with no explicit naming rules. Applying default 'Source'/'Target' suffixes.")
			g.config.NamingRules.SourceSuffix = "Source"
			g.config.NamingRules.TargetSuffix = "Target"
		}
	}

	g.aliasManager.PopulateAliases()

	generatedCode, err := g.generateMainCode(g.typeInfos)
	if err != nil {
		return nil, fmt.Errorf("failed to generate main code: %w", err)
	}

	customStubs, err := g.generateCustomStubs()
	if err != nil {
		return nil, fmt.Errorf("failed to generate custom stubs: %w", err)
	}

	requiredPackages := g.importManager.GetAllImports()

	return &model.GenerationResponse{
		GeneratedCode:    generatedCode,
		CustomStubs:      customStubs,
		RequiredPackages: requiredPackages,
	}, nil
}

func (g *CodeGenerator) generateMainCode(typeInfos map[string]*model.TypeInfo) ([]byte, error) {
	finalBuf := new(bytes.Buffer)

	// Generate all conversion code first to collect required helpers and function bodies.
	conversionFuncs, requiredHelpers, err := g.generateConversionCode(typeInfos)
	if err != nil {
		return nil, err
	}

	// Now, emit all parts in order.
	if err := g.codeEmitter.EmitHeader(finalBuf); err != nil {
		return nil, err
	}
	if err := g.codeEmitter.EmitImports(finalBuf, g.importManager.GetAllImports()); err != nil {
		return nil, err
	}
	if err := g.codeEmitter.EmitAliases(finalBuf); err != nil {
		return nil, err
	}
	if err := g.codeEmitter.EmitConversions(finalBuf, conversionFuncs); err != nil {
		return nil, err
	}
	if err := g.codeEmitter.EmitHelpers(finalBuf, requiredHelpers); err != nil {
		return nil, err
	}

	return format.Source(finalBuf.Bytes())
}

func (g *CodeGenerator) generateConversionCode(typeInfos map[string]*model.TypeInfo) ([]string, map[string]struct{}, error) {
	var conversionFuncs []string
	requiredHelpers := make(map[string]struct{})

	// 使用map来跟踪已生成的函数，避免重复
	generatedFunctions := make(map[string]bool)

	// Generate conversion functions from config rules.
	rules := g.config.ConversionRules
	sort.Slice(rules, func(i, j int) bool { return rules[i].SourceType < rules[j].SourceType })

	for _, rule := range rules {
		sourceInfo := typeInfos[rule.SourceType]
		targetInfo := typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}
		// Generate forward conversion
		g.generateAndCollect(sourceInfo, targetInfo, rule, &conversionFuncs, requiredHelpers, generatedFunctions)

		// Generate reverse conversion if needed
		if rule.Direction == config.DirectionBoth {
			reverseRule := &config.ConversionRule{
				SourceType: targetInfo.FQN(),
				TargetType: sourceInfo.FQN(),
				Direction:  config.DirectionOneway,
				FieldRules: config.FieldRuleSet{Ignore: make(map[string]struct{}), Remap: make(map[string]string)},
			}
			for from, to := range rule.FieldRules.Remap {
				reverseRule.FieldRules.Remap[to] = from
			}
			g.generateAndCollect(targetInfo, sourceInfo, reverseRule, &conversionFuncs, requiredHelpers, generatedFunctions)
		}
	}

	return conversionFuncs, requiredHelpers, nil
}

func (g *CodeGenerator) generateAndCollect(
	sourceInfo, targetInfo *model.TypeInfo,
	rule *config.ConversionRule,
	funcs *[]string,
	helpers map[string]struct{},
	generatedFunctions map[string]bool,
) {
	var generated *model.GeneratedCode
	var err error

	if rule != nil {
		generated, err = g.conversionEngine.GenerateConversionFunction(sourceInfo, targetInfo, rule)
	} else {
		generated, err = g.conversionEngine.GenerateSliceConversion(sourceInfo, targetInfo)
	}

	if err != nil {
		slog.Warn("Error generating conversion function", "source", sourceInfo.FQN(), "target", targetInfo.FQN(), "error", err)
		return
	}

	if generated != nil {
		// 提取函数名，检查是否已生成
		funcName := extractFunctionName(generated.FunctionBody)
		if funcName != "" && !generatedFunctions[funcName] {
			*funcs = append(*funcs, generated.FunctionBody)
			generatedFunctions[funcName] = true
			for _, helper := range generated.RequiredHelpers {
				helpers[helper] = struct{}{}
			}
		}
	}
}

// extractFunctionName 从函数体中提取函数名
func extractFunctionName(functionBody string) string {
	lines := strings.Split(functionBody, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "func ") {
			// 提取函数名，例如：func ConvertUserPPsSourceToUserPPsTarget
			parts := strings.Fields(line)
			if len(parts) >= 2 && strings.HasPrefix(parts[1], "Convert") {
				return parts[1]
			}
		}
	}
	return ""
}

func (g *CodeGenerator) generateCustomStubs() ([]byte, error) {
	// Custom stub generation logic remains the same.
	return nil, nil
}

func (g *CodeGenerator) writeCustomStubsToBuffer(buf *bytes.Buffer, customStubs map[string]string) error {
	// This logic also remains the same.
	return nil
}

func (g *CodeGenerator) discoverImplicitConversionRules(typeInfos map[string]*model.TypeInfo) {
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
				}
				initialRules = append(initialRules, rule)
			}
		}
	}

	g.config.ConversionRules = append(g.config.ConversionRules, initialRules...)

	// New, robust slice rule discovery
	var newSliceRules []*config.ConversionRule
	for _, rule := range initialRules {
		sourceInfo, okS := typeInfos[rule.SourceType]
		targetInfo, okT := typeInfos[rule.TargetType]
		if !okS || !okT {
			continue
		}

		// Rule for []T -> []U
		sourceSliceType := &model.TypeInfo{Kind: model.Slice, Underlying: sourceInfo}
		targetSliceType := &model.TypeInfo{Kind: model.Slice, Underlying: targetInfo}
		typeInfos[sourceSliceType.UniqueKey()] = sourceSliceType
		typeInfos[targetSliceType.UniqueKey()] = targetSliceType
		newSliceRules = append(newSliceRules, &config.ConversionRule{
			SourceType: sourceSliceType.UniqueKey(),
			TargetType: targetSliceType.UniqueKey(),
			Direction:  rule.Direction,
		})

		// Rule for []*T -> []*U
		sourcePtrType := &model.TypeInfo{Kind: model.Pointer, Underlying: sourceInfo}
		targetPtrType := &model.TypeInfo{Kind: model.Pointer, Underlying: targetInfo}
		typeInfos[sourcePtrType.UniqueKey()] = sourcePtrType
		typeInfos[targetPtrType.UniqueKey()] = targetPtrType

		sourcePtrSliceType := &model.TypeInfo{Kind: model.Slice, Underlying: sourcePtrType}
		targetPtrSliceType := &model.TypeInfo{Kind: model.Slice, Underlying: targetPtrType}
		typeInfos[sourcePtrSliceType.UniqueKey()] = sourcePtrSliceType
		typeInfos[targetPtrSliceType.UniqueKey()] = targetPtrSliceType
		newSliceRules = append(newSliceRules, &config.ConversionRule{
			SourceType: sourcePtrSliceType.UniqueKey(),
			TargetType: targetPtrSliceType.UniqueKey(),
			Direction:  rule.Direction,
		})
	}

	g.config.ConversionRules = append(g.config.ConversionRules, newSliceRules...)

	// Sort and remove duplicates
	uniqueRules := make(map[string]*config.ConversionRule)
	for _, rule := range g.config.ConversionRules {
		key := fmt.Sprintf("%s->%s", rule.SourceType, rule.TargetType)
		uniqueRules[key] = rule
	}
	g.config.ConversionRules = make([]*config.ConversionRule, 0, len(uniqueRules))
	for _, rule := range uniqueRules {
		g.config.ConversionRules = append(g.config.ConversionRules, rule)
	}

	sort.Slice(g.config.ConversionRules, func(i, j int) bool {
		if g.config.ConversionRules[i].SourceType != g.config.ConversionRules[j].SourceType {
			return g.config.ConversionRules[i].SourceType < g.config.ConversionRules[j].SourceType
		}
		return g.config.ConversionRules[i].TargetType < g.config.ConversionRules[j].TargetType
	})
}

func (g *CodeGenerator) needsDisambiguation() bool {
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
			return true
		}
	}
	return false
}

func (g *CodeGenerator) getPackageName() string {
	if g.config.GenerationContext.PackageName != "" {
		return g.config.GenerationContext.PackageName
	}
	return "generated"
}

func (g *CodeGenerator) GetConfig() *config.Config {
	return g.config
}
