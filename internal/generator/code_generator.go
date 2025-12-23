package generator

import (
	"bytes"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"golang.org/x/tools/imports"

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
	return &CodeGenerator{
		typeConverter: components.NewTypeConverter(),
	}
}

// initializeComponents configures the generator's components for a specific generation task.
func (g *CodeGenerator) initializeComponents(cfg *config.Config, analysisResult *model.AnalysisResult) {
	g.importManager = components.NewImportManager()
	for alias, path := range cfg.PackageAliases {
		g.importManager.AddAs(path, alias)
	}
	slog.Debug("ImportManager after PackageAliases processing", "imports", g.importManager.GetAllImports())

	g.aliasManager = components.NewAliasManager(cfg, g.importManager, analysisResult.TypeInfos)
	g.nameGenerator = components.NewNameGenerator(g.aliasManager)
	g.typeFormatter = components.NewTypeFormatter(g.aliasManager, g.importManager, analysisResult.TypeInfos)

	g.conversionEngine = components.NewConversionEngine(
		g.typeConverter,
		g.nameGenerator,
		g.typeFormatter,
		g.importManager,
		analysisResult.ExistingFunctions,
	)

	g.codeEmitter = components.NewCodeEmitter(cfg)
}

// Generate executes a complete code generation task.
func (g *CodeGenerator) Generate(cfg *config.Config, analysisResult *model.AnalysisResult) (*model.GenerationResponse, error) {
	finalConfig, activeRules := g.prepareConfigForSession(cfg, analysisResult)
	g.initializeComponents(finalConfig, analysisResult)

	slog.Debug("Components initialized with final configuration.")

	g.aliasManager.PopulateAliases()

	generatedCode, err := g.generateMainCode(finalConfig, analysisResult.TypeInfos, activeRules)
	if err != nil {
		return nil, fmt.Errorf("failed to generate main code: %w", err)
	}

	customStubs, err := g.generateCustomStubs(finalConfig, analysisResult.TypeInfos)
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
func (g *CodeGenerator) prepareConfigForSession(originalCfg *config.Config, analysisResult *model.AnalysisResult) (*config.Config, []*config.ConversionRule) {
	sessionConfig := originalCfg.Clone()
	slog.Debug("Preparing session configuration", "initial_rules", len(sessionConfig.ConversionRules))

	// Populate config with analysis results from the local package
	sessionConfig.ExistingAliases = analysisResult.ExistingAliases

	// Start with implicitly discovered rules
	slog.Debug("Discovering seed rules from package pairs...")
	allRules := g.findSeedRules(analysisResult.TypeInfos, sessionConfig.PackagePairs)

	// Add explicit rules from the config
	allRules = append(allRules, sessionConfig.ConversionRules...)

	// Expand all rules to find dependencies
	activeRules := g.expandRulesByDependencyAnalysis(allRules, analysisResult.TypeInfos)

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
				if g.isFieldMatch(sourceField.Name, tf.Name) {
					targetField = tf
					slog.Debug("Found field match using case-insensitive fallback",
						"sourceField", sourceField.Name, "targetField", targetField.Name)
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

// isFieldMatch performs case-insensitive matching for common field name patterns.
// It returns true if the field names match according to common conventions.
func (g *CodeGenerator) isFieldMatch(sourceName, targetName string) bool {
	// Common pattern: ID vs Id (case-insensitive match)
	if strings.EqualFold(sourceName, targetName) {
		return true
	}

	// Additional common patterns can be added here
	// For now, we focus on the ID/Id case which is the most common

	return false
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
				slog.Debug("Created seed rule", "source", rule.SourceType, "target", rule.TargetType, "direction", rule.Direction)
				seedRules = append(seedRules, rule)
			}
		}
	}
	return seedRules
}

func (g *CodeGenerator) generateMainCode(cfg *config.Config, typeInfos map[string]*model.TypeInfo, rules []*config.ConversionRule) ([]byte, error) {
	finalBuf := new(bytes.Buffer)

	conversionFuncs, requiredHelpers, err := g.generateConversionCode(typeInfos, rules)
	if err != nil {
		return nil, err
	}

	// Prepare the alias information for rendering.
	aliasesToRender := g.prepareAliasesForRender(cfg, typeInfos)

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

	// Collect all helper bodies that need to be emitted.
	var helpersToEmit []model.Helper
	allHelpers := components.GetBuiltInHelpers()
	helperMap := make(map[string]model.Helper)
	for _, h := range allHelpers {
		helperMap[h.Name] = h
	}

	for helperName := range requiredHelpers {
		if h, ok := helperMap[helperName]; ok {
			helpersToEmit = append(helpersToEmit, h)
		}
	}

	if err := g.codeEmitter.EmitHelpers(finalBuf, helpersToEmit); err != nil {
		return nil, err
	}
	process, err := imports.Process("", finalBuf.Bytes(), &imports.Options{
		Fragment:  true,
		Comments:  true,
		TabWidth:  8,
		TabIndent: true,
	})
	if err != nil {
		return nil, err
	}
	return process, nil
}

// prepareAliasesForRender creates the list of aliases to be rendered in the final code.
// This is the gatekeeper that prevents regenerating user-defined aliases.
func (g *CodeGenerator) prepareAliasesForRender(cfg *config.Config, typeInfos map[string]*model.TypeInfo) []*model.AliasRenderInfo {
	var renderInfos []*model.AliasRenderInfo

	// Get the map of types that the AliasManager thinks we need to generate aliases for.
	typesToAlias := g.aliasManager.GetAliasedTypes()

	for uniqueKey, typeInfo := range typesToAlias {
		// Get the alias name that was decided for this type.
		aliasName, ok := g.aliasManager.LookupAlias(uniqueKey)
		if !ok {
			continue
		}

		// --- THE CRITICAL FILTER ---
		// Check if the alias NAME already exists in the user-defined aliases.
		// This is the most direct way to check for user overrides.
		if _, exists := cfg.ExistingAliases[aliasName]; exists {
			slog.Debug("Skipping alias rendering, name already defined by user", "alias", aliasName)
			continue
		}

		originalTypeName := g.typeFormatter.FormatWithoutAlias(typeInfo)

		renderInfos = append(renderInfos, &model.AliasRenderInfo{
			AliasName:        aliasName,
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

	// Initial population of the worklist from the rules.
	// This logic explicitly decomposes bilateral rules into two separate one-way tasks
	// to ensure clarity and prevent any component from misinterpreting a rule's direction.
	for _, rule := range rules {
		sourceInfo := typeInfos[rule.SourceType]
		targetInfo := typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}

		if rule.Direction == config.DirectionBoth {
			slog.Debug("Decomposing bilateral rule into two one-way tasks.", "source", rule.SourceType, "target", rule.TargetType)

			// Create and add the forward task with a cloned, one-way rule.
			forwardRule := *rule // Shallow copy is sufficient here
			forwardRule.Direction = config.DirectionOneway
			worklist = append(worklist, &model.ConversionTask{Source: sourceInfo, Target: targetInfo, Rule: &forwardRule})

			// Create and add the reverse task with a new, inverted one-way rule.
			reverseRule := &config.ConversionRule{
				SourceType: targetInfo.FQN(),
				TargetType: sourceInfo.FQN(),
				Direction:  config.DirectionOneway,
				FieldRules: config.FieldRuleSet{Ignore: make(map[string]struct{}), Remap: make(map[string]string)},
			}
			// Invert field remapping for the reverse direction.
			for from, to := range rule.FieldRules.Remap {
				reverseRule.FieldRules.Remap[to] = from
			}
			worklist = append(worklist, &model.ConversionTask{Source: targetInfo, Target: sourceInfo, Rule: reverseRule})
		} else {
			// For one-way rules, just add the task as is.
			worklist = append(worklist, &model.ConversionTask{Source: sourceInfo, Target: targetInfo, Rule: rule})
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
				requiredHelpers[helper.Name] = struct{}{}
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

func (g *CodeGenerator) generateCustomStubs(cfg *config.Config, typeInfos map[string]*model.TypeInfo) ([]byte, error) {
	stubs := g.conversionEngine.GetStubsToGenerate()
	if len(stubs) == 0 {
		return nil, nil
	}

	// Create a dedicated ImportManager for the stubs file.
	stubImportManager := components.NewImportManager()

	// Create a dedicated TypeFormatter that uses the stubImportManager.
	// We reuse g.aliasManager because type aliases should be consistent,
	// but we need to ensure that if an alias is used, the corresponding import is added to stubImportManager.
	stubTypeFormatter := components.NewTypeFormatter(g.aliasManager, stubImportManager, typeInfos)

	var stubFunctions []string
	var stubNames []string
	for name := range stubs {
		stubNames = append(stubNames, name)
	}
	sort.Strings(stubNames)

	for _, name := range stubNames {
		task := stubs[name]
		// Format types using the stub-specific formatter.
		// This will register necessary imports in stubImportManager.
		sourceTypeStr := stubTypeFormatter.Format(task.Source)
		targetTypeStr := stubTypeFormatter.Format(task.Target)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("// %s is a custom conversion function stub.\n", name))
		sb.WriteString(fmt.Sprintf("// Please implement this function to complete the conversion.\n"))
		sb.WriteString(fmt.Sprintf("func %s(from %s) %s {\n", name, sourceTypeStr, targetTypeStr))
		sb.WriteString(fmt.Sprintf("\t// TODO: Implement this custom conversion\n"))
		sb.WriteString(fmt.Sprintf("\tpanic(\"stub! not implemented\")\n"))
		sb.WriteString("}\n\n")
		stubFunctions = append(stubFunctions, sb.String())
	}

	// Use CodeEmitter to generate the file content
	var finalBuf bytes.Buffer

	// 1. Header
	if err := g.codeEmitter.EmitStubHeader(&finalBuf); err != nil {
		return nil, fmt.Errorf("failed to emit header for stubs: %w", err)
	}

	// 2. Imports
	// Note: We use the stubImportManager's imports, not the main one's.
	if err := g.codeEmitter.EmitImports(&finalBuf, stubImportManager.GetAllImports()); err != nil {
		return nil, fmt.Errorf("failed to emit imports for stubs: %w", err)
	}

	// 3. Conversions (Stubs)
	if err := g.codeEmitter.EmitConversions(&finalBuf, stubFunctions); err != nil {
		return nil, fmt.Errorf("failed to emit stubs: %w", err)
	}

	process, err := imports.Process("", finalBuf.Bytes(), &imports.Options{
		Fragment:  true,
		Comments:  true,
		TabWidth:  8,
		TabIndent: true,
	})
	if err != nil {
		return nil, err
	}
	return process, nil
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
