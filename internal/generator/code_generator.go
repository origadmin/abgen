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

// generationSession holds all the necessary components and state for a single code generation task.
type generationSession struct {
	analysisResult *model.AnalysisResult
	plan           *model.ExecutionPlan
	cfg            *config.Config

	importManager    model.ImportManager
	aliasManager     model.AliasManager
	nameGenerator    model.NameGenerator
	typeFormatter    model.TypeFormatter
	typeConverter    model.TypeConverter
	conversionEngine model.ConversionEngine
	codeEmitter      model.CodeEmitter
}

// Generate is the single public entry point for the generator package.
func Generate(analysisResult *model.AnalysisResult) (*model.GenerationResponse, error) {
	if analysisResult == nil || analysisResult.ExecutionPlan == nil {
		return nil, fmt.Errorf("cannot generate code without a valid analysis result and execution plan")
	}

	session, err := newGenerationSession(analysisResult)
	if err != nil {
		return nil, fmt.Errorf("failed to create generation session: %w", err)
	}

	return session.run()
}

// newGenerationSession creates and initializes a new session with all its components.
func newGenerationSession(analysisResult *model.AnalysisResult) (*generationSession, error) {
	typeConverter := components.NewTypeConverter()
	importManager := components.NewImportManager()
	for alias, path := range analysisResult.ExecutionPlan.FinalConfig.PackageAliases {
		importManager.AddAs(path, alias)
	}

	aliasManager := components.NewAliasManager(analysisResult, importManager)
	nameGenerator := components.NewNameGenerator(aliasManager)
	typeFormatter := components.NewTypeFormatter(analysisResult, aliasManager, importManager)
	codeEmitter := components.NewCodeEmitter(analysisResult)

	conversionEngine := components.NewConversionEngine(
		analysisResult,
		typeConverter,
		nameGenerator,
		typeFormatter,
		importManager,
	)

	return &generationSession{
		analysisResult:   analysisResult,
		plan:             analysisResult.ExecutionPlan,
		cfg:              analysisResult.ExecutionPlan.FinalConfig,
		typeConverter:    typeConverter,
		importManager:    importManager,
		aliasManager:     aliasManager,
		nameGenerator:    nameGenerator,
		typeFormatter:    typeFormatter,
		conversionEngine: conversionEngine,
		codeEmitter:      codeEmitter,
	}, nil
}

// run executes the main generation logic for the session.
func (s *generationSession) run() (*model.GenerationResponse, error) {
	slog.Debug("Generator session: Running...")

	s.aliasManager.PopulateAliases()

	generatedCode, err := s.generateMainCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate main code: %w", err)
	}

	customStubs, err := s.generateCustomStubs()
	if err != nil {
		return nil, fmt.Errorf("failed to generate custom stubs: %w", err)
	}

	importMap := s.importManager.GetAllImports()
	requiredPackages := make([]string, 0, len(importMap))
	for pkgPath := range importMap {
		requiredPackages = append(requiredPackages, pkgPath)
	}
	sort.Strings(requiredPackages)

	slog.Debug("Generator session: Finished.")
	return &model.GenerationResponse{
		GeneratedCode:    generatedCode,
		CustomStubs:      customStubs,
		RequiredPackages: requiredPackages,
	}, nil
}

// generateMainCode produces the primary generated file content.
func (s *generationSession) generateMainCode() ([]byte, error) {
	finalBuf := new(bytes.Buffer)

	conversionFuncs, requiredHelpers, err := s.generateConversionCode()
	if err != nil {
		return nil, err
	}

	aliasesToRender := s.prepareAliasesForRender()

	if err := s.codeEmitter.EmitHeader(finalBuf); err != nil {
		return nil, err
	}
	if err := s.codeEmitter.EmitImports(finalBuf, s.importManager.GetAllImports()); err != nil {
		return nil, err
	}
	if err := s.codeEmitter.EmitAliases(finalBuf, aliasesToRender); err != nil {
		return nil, err
	}
	if err := s.codeEmitter.EmitConversions(finalBuf, conversionFuncs); err != nil {
		return nil, err
	}

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

	if err := s.codeEmitter.EmitHelpers(finalBuf, helpersToEmit); err != nil {
		return nil, err
	}

	return imports.Process("", finalBuf.Bytes(), &imports.Options{
		Fragment:  true,
		Comments:  true,
		TabWidth:  8,
		TabIndent: true,
	})
}

// prepareAliasesForRender creates the list of aliases to be rendered in the final code.
func (s *generationSession) prepareAliasesForRender() []*model.AliasRenderInfo {
	var renderInfos []*model.AliasRenderInfo
	typesToAlias := s.aliasManager.GetAliasedTypes()

	for uniqueKey, typeInfo := range typesToAlias {
		aliasName, ok := s.aliasManager.LookupAlias(uniqueKey)
		if !ok {
			continue
		}

		if _, exists := s.analysisResult.ExistingAliases[aliasName]; exists {
			slog.Debug("Skipping alias rendering, name already defined by user", "alias", aliasName)
			continue
		}

		originalTypeName := s.typeFormatter.FormatWithoutAlias(typeInfo)

		if aliasName == originalTypeName {
			slog.Debug("Skipping self-referencing alias", "alias", aliasName, "original", originalTypeName)
			continue
		}

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

// generateConversionCode generates all necessary conversion functions.
func (s *generationSession) generateConversionCode() ([]string, map[string]struct{}, error) {
	var conversionFuncs []string
	requiredHelpers := make(map[string]struct{})
	generatedFunctions := make(map[string]bool)
	worklist := make([]*model.ConversionTask, 0)

	for _, rule := range s.plan.ActiveRules {
		sourceInfo := s.analysisResult.TypeInfos[rule.SourceType]
		targetInfo := s.analysisResult.TypeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}

		if rule.Direction == config.DirectionBoth {
			forwardRule := *rule
			forwardRule.Direction = config.DirectionOneway
			worklist = append(worklist, &model.ConversionTask{Source: sourceInfo, Target: targetInfo, Rule: &forwardRule})

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
		} else {
			worklist = append(worklist, &model.ConversionTask{Source: sourceInfo, Target: targetInfo, Rule: rule})
		}
	}

	for len(worklist) > 0 {
		task := worklist[0]
		worklist = worklist[1:]

		funcName := s.nameGenerator.ConversionFunctionName(task.Source, task.Target)
		if generatedFunctions[funcName] {
			continue
		}

		generated, newTasks, err := s.conversionEngine.GenerateConversionFunction(task.Source, task.Target, task.Rule)
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
			for _, newTask := range newTasks {
				newFuncName := s.nameGenerator.ConversionFunctionName(newTask.Source, newTask.Target)
				if !generatedFunctions[newFuncName] {
					worklist = append(worklist, newTask)
				}
			}
		}
	}

	sort.Strings(conversionFuncs)
	return conversionFuncs, requiredHelpers, nil
}

// generateCustomStubs produces a file with function stubs for custom conversions.
func (s *generationSession) generateCustomStubs() ([]byte, error) {
	stubs := s.conversionEngine.GetStubsToGenerate()
	if len(stubs) == 0 {
		return nil, nil
	}

	// Create a dedicated ImportManager and TypeFormatter for the stubs file.
	// This ensures that only the necessary imports for the stubs are included.
	stubImportManager := components.NewImportManager()
	stubTypeFormatter := components.NewTypeFormatter(s.analysisResult, s.aliasManager, stubImportManager)

	var stubFunctions []string
	var stubNames []string
	for name := range stubs {
		stubNames = append(stubNames, name)
	}
	sort.Strings(stubNames)

	for _, name := range stubNames {
		task := stubs[name]
		// Formatting the types here will automatically add their packages to the stubImportManager.
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

	var finalBuf bytes.Buffer
	if err := s.codeEmitter.EmitStubHeader(&finalBuf); err != nil {
		return nil, fmt.Errorf("failed to emit header for stubs: %w", err)
	}
	if err := s.codeEmitter.EmitImports(&finalBuf, stubImportManager.GetAllImports()); err != nil {
		return nil, fmt.Errorf("failed to emit imports for stubs: %w", err)
	}
	if err := s.codeEmitter.EmitConversions(&finalBuf, stubFunctions); err != nil {
		return nil, fmt.Errorf("failed to emit stubs: %w", err)
	}

	return imports.Process("", finalBuf.Bytes(), &imports.Options{
		Fragment:  true,
		Comments:  true,
		TabWidth:  8,
		TabIndent: true,
	})
}
