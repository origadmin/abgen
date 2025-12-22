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

// CodeGenerator orchestrates the entire code generation process.
type CodeGenerator struct {
	importManager    model.ImportManager
	nameGenerator    model.NameGenerator
	aliasManager     model.AliasManager
	conversionEngine model.ConversionEngine
	codeEmitter      model.CodeEmitter
	typeConverter    model.TypeConverter
	typeFormatter    *components.TypeFormatter
}

// NewCodeGenerator creates a new, stateless code generator.
func NewCodeGenerator() model.CodeGenerator {
	// TypeConverter is stateless, so it can be created once.
	return &CodeGenerator{
		typeConverter: components.NewTypeConverter(),
	}
}

// initializeComponents configures the generator's components for a specific generation task.
func (g *CodeGenerator) initializeComponents(cfg *config.Config, typeInfos map[string]*model.TypeInfo) {
	// Create components in dependency order.
	// 1. Components with no internal dependencies.
	g.importManager = components.NewImportManager()

	// Add required imports directly from PackageAliases
	for alias, path := range cfg.PackageAliases {
		g.importManager.AddAs(path, alias)
	}
	slog.Debug("ImportManager after PackageAliases processing", "imports", g.importManager.GetAllImports())

	g.nameGenerator = components.NewNameGenerator(cfg)
	// g.typeConverter is already initialized in NewCodeGenerator.

	// 2. AliasManager depends on ImportManager.
	// We perform a type assertion because we know the concrete type and need it for the next step.
	aliasManager := components.NewAliasManager(cfg, g.importManager, typeInfos).(*components.AliasManager)
	g.aliasManager = aliasManager

	// 3. TypeFormatter depends on AliasManager and ImportManager.
	g.typeFormatter = components.NewTypeFormatter(g.aliasManager, g.importManager, typeInfos)

	// 4. ConversionEngine depends on several components.
	g.conversionEngine = components.NewConversionEngine(
		g.typeConverter,
		g.nameGenerator,
		g.typeFormatter,
		g.importManager,
	)

	// 5. CodeEmitter is simplified and only depends on config.
	g.codeEmitter = components.NewCodeEmitter(cfg)
}

// Generate executes a complete code generation task.
func (g *CodeGenerator) Generate(cfg *config.Config, typeInfos map[string]*model.TypeInfo) (*model.GenerationResponse, error) {
	finalConfig, activeRules := g.prepareConfigForSession(cfg, typeInfos)
	g.initializeComponents(finalConfig, typeInfos)

	slog.Debug("Components initialized with final configuration.")

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

// prepareConfigForSession creates a finalized configuration for a single generation run.
func (g *CodeGenerator) prepareConfigForSession(originalCfg *config.Config, typeInfos map[string]*model.TypeInfo) (*config.Config, []*config.ConversionRule) {
	sessionConfig := deepCopyConfig(originalCfg)
	slog.Debug("Preparing session configuration", "initial_rules", len(sessionConfig.ConversionRules))

	var activeRules []*config.ConversionRule
	if len(sessionConfig.ConversionRules) > 0 {
		activeRules = g.expandRulesByDependencyAnalysis(sessionConfig.ConversionRules, typeInfos)
	} else {
		slog.Debug("No explicit conversion rules found, discovering seed rules from package pairs...")
		seedRules := g.findSeedRules(typeInfos, sessionConfig.PackagePairs)
		activeRules = g.expandRulesByDependencyAnalysis(seedRules, typeInfos)
	}
	slog.Debug("Rule expansion finished", "total_active_rules", len(activeRules))

	if sessionConfig.NamingRules.SourcePrefix == "" && sessionConfig.NamingRules.SourceSuffix == "" &&
		sessionConfig.NamingRules.TargetPrefix == "" && sessionConfig.NamingRules.TargetSuffix == "" {
		if g.needsDisambiguation(activeRules) {
			slog.Debug("Ambiguous type names found, applying default 'Source'/'Target' suffixes for this session.")
			sessionConfig.NamingRules.SourceSuffix = "Source"
			sessionConfig.NamingRules.TargetSuffix = "Target"
		}
	}

	sessionConfig.ConversionRules = activeRules
	return sessionConfig, activeRules
}

// expandRulesByDependencyAnalysis takes a set of entry-point rules and discovers all
// transitive dependencies by analyzing struct fields.
func (g *CodeGenerator) expandRulesByDependencyAnalysis(seedRules []*config.ConversionRule, typeInfos map[string]*model.TypeInfo) []*config.ConversionRule {
	allKnownRules := make(map[string]*config.ConversionRule)
	worklist := make([]*config.ConversionRule, 0, len(seedRules))

	for _, rule := range seedRules {
		key := fmt.Sprintf("%s->%s", rule.SourceType, rule.TargetType)
		if _, exists := allKnownRules[key]; !exists {
			allKnownRules[key] = rule
			worklist = append(worklist, rule)
		}
	}

	for len(worklist) > 0 {
		rule := worklist[0]
		worklist = worklist[1:]

		sourceInfo := typeInfos[rule.SourceType]
		targetInfo := typeInfos[rule.TargetType]

		if sourceInfo == nil || targetInfo == nil || sourceInfo.Kind != model.Struct || targetInfo.Kind != model.Struct {
			continue
		}

		for _, sourceField := range sourceInfo.Fields {
			targetFieldName := sourceField.Name
			if remappedName, ok := rule.FieldRules.Remap[sourceField.Name]; ok {
				targetFieldName = remappedName
			}

			var targetField *model.FieldInfo
			for _, tf := range targetInfo.Fields {
				if tf.Name == targetFieldName {
					targetField = tf
					break
				}
			}

			if targetField == nil {
				continue
			}

			baseSourceType := g.typeConverter.GetElementType(sourceField.Type)
			baseTargetType := g.typeConverter.GetElementType(targetField.Type)

			if baseSourceType == nil || baseTargetType == nil || baseSourceType.UniqueKey() == baseTargetType.UniqueKey() {
				continue
			}

			newRule := &config.ConversionRule{
				SourceType: baseSourceType.UniqueKey(),
				TargetType: baseTargetType.UniqueKey(),
				Direction:  config.DirectionBoth,
			}

			key := fmt.Sprintf("%s->%s", newRule.SourceType, newRule.TargetType)
			if _, exists := allKnownRules[key]; !exists {
				allKnownRules[key] = newRule
				worklist = append(worklist, newRule)
				slog.Debug("Discovered new dependency rule", "source", newRule.SourceType, "target", newRule.TargetType)
			}
		}
	}

	finalRules := make([]*config.ConversionRule, 0, len(allKnownRules))
	for _, rule := range allKnownRules {
		finalRules = append(finalRules, rule)
	}
	sort.Slice(finalRules, func(i, j int) bool {
		return finalRules[i].SourceType < finalRules[j].SourceType
	})
	return finalRules
}

// findSeedRules finds the initial set of conversion rules based on matching type names.
func (g *CodeGenerator) findSeedRules(typeInfos map[string]*model.TypeInfo, packagePairs []*config.PackagePair) []*config.ConversionRule {
	var seedRules []*config.ConversionRule
	typesByPackage := make(map[string][]*model.TypeInfo)
	for _, info := range typeInfos {
		if info.IsNamedType() {
			typesByPackage[info.ImportPath] = append(typesByPackage[info.ImportPath], info)
		}
	}

	for _, pair := range packagePairs {
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
				seedRules = append(seedRules, rule)
			}
		}
	}
	return seedRules
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
		PackageAliases:    make(map[string]string, len(original.PackageAliases)),    // Deep copy PackageAliases
		CustomFunctionRules: make(map[string]string, len(original.CustomFunctionRules)), // Deep copy CustomFunctionRules
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

	// Deep copy PackageAliases
	for k, v := range original.PackageAliases {
		cpy.PackageAliases[k] = v
	}

	// Deep copy CustomFunctionRules
	for k, v := range original.CustomFunctionRules {
		cpy.CustomFunctionRules[k] = v
	}

	return cpy
}

func (g *CodeGenerator) generateMainCode(typeInfos map[string]*model.TypeInfo, rules []*config.ConversionRule) ([]byte, error) {
	finalBuf := new(bytes.Buffer)

	conversionFuncs, requiredHelpers, err := g.generateConversionCode(typeInfos, rules)
	if err != nil {
		return nil, err
	}

	// Prepare the alias information for rendering.
	aliasesToRender := g.prepareAliasesForRender(typeInfos)

	if err := g.codeEmitter.EmitHeader(finalBuf); err != nil {
		return nil, err
	}
	if err := g.codeEmitter.EmitImports(finalBuf, g.importManager.GetAllImports()); err != nil {
		return nil, err
	}
	if err := g.codeEmitter.EmitAliases(finalBuf, aliasesToRender); err != nil {
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

// prepareAliasesForRender creates the list of aliases to be rendered in the final code.
func (g *CodeGenerator) prepareAliasesForRender(typeInfos map[string]*model.TypeInfo) []*model.AliasRenderInfo {
	var renderInfos []*model.AliasRenderInfo
	allAliases := g.aliasManager.GetAllAliases()

	for uniqueKey, alias := range allAliases {
		var originalTypeName string
		typeInfo, ok := typeInfos[uniqueKey]
		if !ok {
			slog.Debug("prepareAliasesForRender: TypeInfo not found for key, creating temporary one.", "uniqueKey", uniqueKey)
			typeInfo = g.reconstructTypeInfoFromKey(uniqueKey, typeInfos)
		}

		if typeInfo != nil {
			originalTypeName = g.typeFormatter.FormatWithoutAlias(typeInfo)
		} else {
			slog.Warn("prepareAliasesForRender: Could not reconstruct TypeInfo, falling back to uniqueKey.", "uniqueKey", uniqueKey)
			originalTypeName = uniqueKey // This should not happen with the new reconstructor
		}

		renderInfos = append(renderInfos, &model.AliasRenderInfo{
			AliasName:        alias,
			OriginalTypeName: originalTypeName,
		})
	}

	sort.Slice(renderInfos, func(i, j int) bool {
		return renderInfos[i].AliasName < renderInfos[j].AliasName
	})

	return renderInfos
}

// reconstructTypeInfoFromKey attempts to build a temporary TypeInfo from a unique key string.
func (g *CodeGenerator) reconstructTypeInfoFromKey(key string, typeInfos map[string]*model.TypeInfo) *model.TypeInfo {
	if info, ok := typeInfos[key]; ok {
		return info
	}

	if strings.HasPrefix(key, "[]") {
		underlying := g.reconstructTypeInfoFromKey(key[2:], typeInfos)
		if underlying != nil {
			return &model.TypeInfo{Kind: model.Slice, Underlying: underlying}
		}
	} else if strings.HasPrefix(key, "*") {
		underlying := g.reconstructTypeInfoFromKey(key[1:], typeInfos)
		if underlying != nil {
			return &model.TypeInfo{Kind: model.Pointer, Underlying: underlying}
		}
	}

	// It's a named type that wasn't in the original map.
	// Create a partial TypeInfo for it.
	lastDot := strings.LastIndex(key, ".")
	if lastDot != -1 {
		pkgPath := key[:lastDot]
		typeName := key[lastDot+1:]
		return &model.TypeInfo{
			Name:       typeName,
			ImportPath: pkgPath,
			Kind:       model.Named, // Assume it's a named type
		}
	}

	// It's a primitive type
	if !strings.Contains(key, ".") {
		return &model.TypeInfo{
			Name: key,
			Kind: model.Primitive,
		}
	}

	slog.Warn("reconstructTypeInfoFromKey: could not fully reconstruct type", "key", key)
	return nil
}


func (g *CodeGenerator) generateConversionCode(typeInfos map[string]*model.TypeInfo, rules []*config.ConversionRule) ([]string, map[string]struct{}, error) {
	var conversionFuncs []string
	requiredHelpers := make(map[string]struct{})
	generatedFunctions := make(map[string]bool)
	worklist := make([]*model.ConversionTask, 0)

	// Initial population of the worklist from the rules
	for _, rule := range rules {
		sourceInfo := typeInfos[rule.SourceType]
		targetInfo := typeInfos[rule.TargetType]
		if sourceInfo != nil && targetInfo != nil {
			worklist = append(worklist, &model.ConversionTask{Source: sourceInfo, Target: targetInfo, Rule: rule})
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
				worklist = append(worklist, &model.ConversionTask{Source: targetInfo, Target: sourceInfo, Rule: reverseRule})
			}
		}
	}

	for len(worklist) > 0 {
		task := worklist[0]
		worklist = worklist[1:]

		funcName := g.nameGenerator.ConversionFunctionName(task.Source, task.Target)
		if generatedFunctions[funcName] {
			continue
		}

		generated, newTasks, err := g.conversionEngine.GenerateConversionFunction(task.Source, task.Target, task.Rule)
		if err != nil {
			slog.Warn("Error generating conversion function", "source", task.Source.FQN(), "target", task.Target.FQN(), "error", err)
			continue
		}

		if generated != nil {
			conversionFuncs = append(conversionFuncs, generated.FunctionBody)
			generatedFunctions[funcName] = true
			for _, helper := range generated.RequiredHelpers {
				requiredHelpers[helper] = struct{}{}
			}
			// Add newly discovered tasks to the worklist
			for _, newTask := range newTasks {
				newFuncName := g.nameGenerator.ConversionFunctionName(newTask.Source, newTask.Target)
				if !generatedFunctions[newFuncName] {
					worklist = append(worklist, newTask)
				}
			}
		}
	}

	sort.Strings(conversionFuncs)
	return conversionFuncs, requiredHelpers, nil
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
