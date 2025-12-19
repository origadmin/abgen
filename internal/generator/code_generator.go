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
	importManager    model.ImportManager
	nameGenerator    model.NameGenerator
	aliasManager     model.AliasManager
	conversionEngine model.ConversionEngine
	codeEmitter      model.CodeEmitter
	typeConverter    model.TypeConverter
}

// NewCodeGenerator creates a new, stateless code generator.
func NewCodeGenerator() model.CodeGenerator {
	return &CodeGenerator{
		typeConverter: components.NewTypeConverter(),
	}
}

// initializeComponents configures the generator's components for a specific generation task.
func (g *CodeGenerator) initializeComponents(cfg *config.Config, typeInfos map[string]*model.TypeInfo) {
	g.importManager = components.NewImportManager()
	g.nameGenerator = components.NewNameGenerator(cfg, g.importManager)
	g.aliasManager = components.NewAliasManager(cfg, g.importManager, g.nameGenerator, typeInfos)
	g.conversionEngine = components.NewConversionEngine(
		g.typeConverter,
		g.nameGenerator,
		g.aliasManager,
		g.importManager,
	)
	g.codeEmitter = components.NewCodeEmitter(
		cfg,
		g.importManager,
		g.aliasManager,
	)
}

// Generate executes a complete code generation task.
func (g *CodeGenerator) Generate(cfg *config.Config, typeInfos map[string]*model.TypeInfo) (*model.GenerationResponse, error) {
	sessionConfig := deepCopyConfig(cfg)
	g.initializeComponents(sessionConfig, typeInfos)

	slog.Debug("Generating code with orchestrator",
		"type_count", len(typeInfos),
		"initial_rules", len(sessionConfig.ConversionRules))

	activeRules := sessionConfig.ConversionRules
	if len(activeRules) == 0 {
		slog.Debug("No explicit conversion rules found, discovering implicit rules...")
		discoveredRules := g.discoverImplicitRules(typeInfos, sessionConfig.PackagePairs)
		activeRules = append(activeRules, discoveredRules...)
		slog.Debug("Implicit rule discovery finished", "discovered_rules", len(discoveredRules))
	}

	if sessionConfig.NamingRules.SourcePrefix == "" && sessionConfig.NamingRules.SourceSuffix == "" &&
		sessionConfig.NamingRules.TargetPrefix == "" && sessionConfig.NamingRules.TargetSuffix == "" {
		if g.needsDisambiguation(activeRules) {
			slog.Debug("Ambiguous type names found, applying default 'Source'/'Target' suffixes for this session.")
			sessionConfig.NamingRules.SourceSuffix = "Source"
			sessionConfig.NamingRules.TargetSuffix = "Target"
			g.initializeComponents(sessionConfig, typeInfos)
		}
	}

	g.aliasManager.PopulateAliases()

	generatedCode, err := g.generateMainCode(typeInfos, activeRules)
	if err != nil {
		return nil, fmt.Errorf("failed to generate main code: %w", err)
	}

	customStubs, err := g.generateCustomStubs()
	if err != nil {
		return nil, fmt.Errorf("failed to generate custom stubs: %w", err)
	}

	importMap := g.importManager.GetAllImports()
	requiredPackages := make([]string, 0, len(importMap))
	for pkgPath := range importMap {
		requiredPackages = append(requiredPackages, pkgPath)
	}
	sort.Strings(requiredPackages)

	return &model.GenerationResponse{
		GeneratedCode:    generatedCode,
		CustomStubs:      customStubs,
		RequiredPackages: requiredPackages,
	}, nil
}

func deepCopyConfig(original *config.Config) *config.Config {
	if original == nil {
		return nil
	}

	cpy := &config.Config{
		NamingRules:       original.NamingRules,
		GenerationContext: original.GenerationContext,
		PackagePairs:      make([]*config.PackagePair, len(original.PackagePairs)),
		ConversionRules:   make([]*config.ConversionRule, 0, len(original.ConversionRules)),
	}

	for i, pair := range original.PackagePairs {
		if pair != nil {
			pairCopy := *pair
			cpy.PackagePairs[i] = &pairCopy
		}
	}

	for _, rule := range original.ConversionRules {
		if rule != nil {
			ruleCopy := &config.ConversionRule{
				SourceType: rule.SourceType,
				TargetType: rule.TargetType,
				Direction:  rule.Direction,
				FieldRules: config.FieldRuleSet{
					Ignore: make(map[string]struct{}, len(rule.FieldRules.Ignore)),
					Remap:  make(map[string]string, len(rule.FieldRules.Remap)),
				},
			}
			for k, v := range rule.FieldRules.Ignore {
				ruleCopy.FieldRules.Ignore[k] = v
			}
			for k, v := range rule.FieldRules.Remap {
				ruleCopy.FieldRules.Remap[k] = v
			}
			cpy.ConversionRules = append(cpy.ConversionRules, ruleCopy)
		}
	}

	return cpy
}

func (g *CodeGenerator) generateMainCode(typeInfos map[string]*model.TypeInfo, rules []*config.ConversionRule) ([]byte, error) {
	finalBuf := new(bytes.Buffer)

	conversionFuncs, requiredHelpers, err := g.generateConversionCode(typeInfos, rules)
	if err != nil {
		return nil, err
	}

	if err := g.codeEmitter.EmitHeader(finalBuf); err != nil {
		return nil, err
	}
	if err := g.codeEmitter.EmitImports(finalBuf, g.importManager.GetAllImports()); err != nil {
		return nil, err
	}
	// Pass the rendered aliases to the emitter.
	if err := g.codeEmitter.EmitAliases(finalBuf, g.aliasManager.GetAliasesToRender()); err != nil {
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

func (g *CodeGenerator) generateConversionCode(typeInfos map[string]*model.TypeInfo, rules []*config.ConversionRule) ([]string, map[string]struct{}, error) {
	var conversionFuncs []string
	requiredHelpers := make(map[string]struct{})
	generatedFunctions := make(map[string]bool)

	sort.Slice(rules, func(i, j int) bool { return rules[i].SourceType < rules[j].SourceType })

	for _, rule := range rules {
		sourceInfo := typeInfos[rule.SourceType]
		targetInfo := typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}
		g.generateAndCollect(sourceInfo, targetInfo, rule, &conversionFuncs, requiredHelpers, generatedFunctions)

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

func extractFunctionName(functionBody string) string {
	lines := strings.Split(functionBody, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "func ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && strings.HasPrefix(parts[1], "Convert") {
				return parts[1]
			}
		}
	}
	return ""
}

func (g *CodeGenerator) generateCustomStubs() ([]byte, error) {
	return nil, nil
}

func (g *CodeGenerator) discoverImplicitRules(typeInfos map[string]*model.TypeInfo, packagePairs []*config.PackagePair) []*config.ConversionRule {
	typesByPackage := make(map[string][]*model.TypeInfo)
	for _, info := range typeInfos {
		if info.IsNamedType() {
			typesByPackage[info.ImportPath] = append(typesByPackage[info.ImportPath], info)
		}
	}

	var discoveredRules []*config.ConversionRule
	for _, pair := range packagePairs {
		if pair == nil {
			continue
		}
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
				discoveredRules = append(discoveredRules, rule)
			}
		}
	}

	var sliceRules []*config.ConversionRule
	for _, rule := range discoveredRules {
		sourceInfo, okS := typeInfos[rule.SourceType]
		targetInfo, okT := typeInfos[rule.TargetType]
		if !okS || !okT {
			continue
		}

		sourceSliceType := &model.TypeInfo{Kind: model.Slice, Underlying: sourceInfo}
		targetSliceType := &model.TypeInfo{Kind: model.Slice, Underlying: targetInfo}
		sliceRules = append(sliceRules, &config.ConversionRule{
			SourceType: sourceSliceType.UniqueKey(),
			TargetType: targetSliceType.UniqueKey(),
			Direction:  rule.Direction,
		})

		sourcePtrType := &model.TypeInfo{Kind: model.Pointer, Underlying: sourceInfo}
		targetPtrType := &model.TypeInfo{Kind: model.Pointer, Underlying: targetInfo}
		sourcePtrSliceType := &model.TypeInfo{Kind: model.Slice, Underlying: sourcePtrType}
		targetPtrSliceType := &model.TypeInfo{Kind: model.Slice, Underlying: targetPtrType}
		sliceRules = append(sliceRules, &config.ConversionRule{
			SourceType: sourcePtrSliceType.UniqueKey(),
			TargetType: targetPtrSliceType.UniqueKey(),
			Direction:  rule.Direction,
		})
	}

	allDiscoveredRules := append(discoveredRules, sliceRules...)

	uniqueRules := make(map[string]*config.ConversionRule)
	for _, rule := range allDiscoveredRules {
		key := fmt.Sprintf("%s->%s", rule.SourceType, rule.TargetType)
		uniqueRules[key] = rule
	}

	finalRules := make([]*config.ConversionRule, 0, len(uniqueRules))
	for _, rule := range uniqueRules {
		finalRules = append(finalRules, rule)
	}

	return finalRules
}

func (g *CodeGenerator) needsDisambiguation(rules []*config.ConversionRule) bool {
	for _, rule := range rules {
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
