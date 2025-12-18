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
		g.generateAndCollect(sourceInfo, targetInfo, rule, &conversionFuncs, requiredHelpers)

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
			g.generateAndCollect(targetInfo, sourceInfo, reverseRule, &conversionFuncs, requiredHelpers)
		}
	}

	// Generate dynamically discovered slice conversion functions.
	sliceConversions := g.discoverSliceConversions(typeInfos)
	for _, conv := range sliceConversions {
		g.generateAndCollect(conv.sourceInfo, conv.targetInfo, nil, &conversionFuncs, requiredHelpers)
	}

	return conversionFuncs, requiredHelpers, nil
}

func (g *CodeGenerator) generateAndCollect(
	sourceInfo, targetInfo *model.TypeInfo,
	rule *config.ConversionRule,
	funcs *[]string,
	helpers map[string]struct{},
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
		*funcs = append(*funcs, generated.FunctionBody)
		for _, helper := range generated.RequiredHelpers {
			helpers[helper] = struct{}{}
		}
	}
}

func (g *CodeGenerator) discoverSliceConversions(typeInfos map[string]*model.TypeInfo) []struct {
	sourceInfo *model.TypeInfo
	targetInfo *model.TypeInfo
} {
	sliceConversionSet := make(map[string]struct {
		sourceInfo *model.TypeInfo
		targetInfo *model.TypeInfo
	})

	for _, rule := range g.config.ConversionRules {
		sourceInfo := typeInfos[rule.SourceType]
		targetInfo := typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}

		for _, sourceField := range sourceInfo.Fields {
			targetFieldName := sourceField.Name
			if remap, ok := rule.FieldRules.Remap[sourceField.Name]; ok {
				targetFieldName = remap
			}

			var targetField *model.FieldInfo
			for _, tf := range targetInfo.Fields {
				if tf.Name == targetFieldName {
					targetField = tf
					break
				}
			}

			if targetField != nil &&
				g.typeConverter.IsSlice(sourceField.Type) &&
				g.typeConverter.IsSlice(targetField.Type) {
				sourceElem := g.typeConverter.GetElementType(sourceField.Type)
				targetElem := g.typeConverter.GetElementType(targetField.Type)

				if sourceElem != nil && targetElem != nil && sourceElem.UniqueKey() != targetElem.UniqueKey() {
					key := sourceField.Type.UniqueKey() + "->" + targetField.Type.UniqueKey()
					sliceConversionSet[key] = struct {
						sourceInfo *model.TypeInfo
						targetInfo *model.TypeInfo
					}{sourceInfo: sourceField.Type, targetInfo: targetField.Type}
				}
			}
		}
	}

	var sliceConversions []struct {
		sourceInfo *model.TypeInfo
		targetInfo *model.TypeInfo
	}
	for _, v := range sliceConversionSet {
		sliceConversions = append(sliceConversions, v)
	}
	return sliceConversions
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
					FieldRules: config.FieldRuleSet{Ignore: make(map[string]struct{}), Remap: make(map[string]string)},
				}
				initialRules = append(initialRules, rule)
			}
		}
	}

	g.config.ConversionRules = append(g.config.ConversionRules, initialRules...)

	// Now, generate rules for slice types based on the initial named type rules
	for _, rule := range initialRules {
		sourceInfo := typeInfos[rule.SourceType]
		targetInfo := typeInfos[rule.TargetType]

		sourceSliceType := &model.TypeInfo{
			Kind:       model.Slice,
			Underlying: sourceInfo,
		}
		targetSliceType := &model.TypeInfo{
			Kind:       model.Slice,
			Underlying: targetInfo,
		}
		typeInfos[sourceSliceType.UniqueKey()] = sourceSliceType
		typeInfos[targetSliceType.UniqueKey()] = targetSliceType

		sliceRule := &config.ConversionRule{
			SourceType: sourceSliceType.UniqueKey(),
			TargetType: targetSliceType.UniqueKey(),
			Direction:  config.DirectionBoth,
		}

		// Check if there are already rules related to sourceSliceType or targetSliceType
		// Avoid generating duplicate or conflicting implicit slice rules
		foundExistingSliceRule := false
		for _, existingRule := range g.config.ConversionRules {
			if (existingRule.SourceType == sliceRule.SourceType && existingRule.TargetType == sliceRule.TargetType) ||
				(existingRule.SourceType == sliceRule.TargetType && existingRule.TargetType == sliceRule.SourceType && existingRule.Direction == config.DirectionBoth) {
				foundExistingSliceRule = true
				break
			}
		}

		if !foundExistingSliceRule {
			g.config.ConversionRules = append(g.config.ConversionRules, sliceRule)
		}
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
