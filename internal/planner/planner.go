package planner

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// Planner implements the model.Planner interface.
type Planner struct {
	typeConverter model.TypeConverter
}

// NewPlanner creates a new planner.
func NewPlanner(typeConverter model.TypeConverter) model.Planner {
	return &Planner{
		typeConverter: typeConverter,
	}
}

// Plan creates the final execution plan for the generator.
func (p *Planner) Plan(initialConfig *config.Config, typeInfos map[string]*model.TypeInfo) *model.ExecutionPlan {
	finalConfig := initialConfig.Clone()
	slog.Debug("Planner: Starting to create execution plan", "initial_rules", len(finalConfig.ConversionRules))

	// Start with implicitly discovered rules from package pairs
	allRules := p.findSeedRules(typeInfos, finalConfig.PackagePairs)

	// Add explicit rules from the config
	allRules = append(allRules, finalConfig.ConversionRules...)

	// Expand all rules to find dependencies by analyzing struct fields
	activeRules := p.expandRulesByDependencyAnalysis(allRules, typeInfos)

	slog.Debug("Planner: Rule expansion finished", "total_active_rules", len(activeRules))

	// Apply default naming rules if necessary to avoid conflicts
	if finalConfig.NamingRules.SourcePrefix == "" && finalConfig.NamingRules.SourceSuffix == "" &&
		finalConfig.NamingRules.TargetPrefix == "" && finalConfig.NamingRules.TargetSuffix == "" {
		if p.needsDisambiguation(activeRules) {
			slog.Debug("Planner: Ambiguous type names found, applying default 'Source'/'Target' suffixes.")
			finalConfig.NamingRules.SourceSuffix = "Source"
			finalConfig.NamingRules.TargetSuffix = "Target"
		}
	}

	finalConfig.ConversionRules = activeRules

	return &model.ExecutionPlan{
		FinalConfig: finalConfig,
		ActiveRules: activeRules,
	}
}

// expandRulesByDependencyAnalysis discovers all transitive dependencies by analyzing struct fields.
func (p *Planner) expandRulesByDependencyAnalysis(seedRules []*config.ConversionRule, typeInfos map[string]*model.TypeInfo) []*config.ConversionRule {
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

		if sourceInfo == nil || targetInfo == nil || !sourceInfo.IsUltimatelyStruct() || !targetInfo.IsUltimatelyStruct() {
			continue
		}

		for _, sourceField := range sourceInfo.Fields {
			targetFieldName := sourceField.Name
			if remappedName, ok := rule.FieldRules.Remap[sourceField.Name]; ok {
				targetFieldName = remappedName
			}

			var targetField *model.FieldInfo
			for _, tf := range targetInfo.Fields {
				if tf.Name == targetFieldName || strings.EqualFold(tf.Name, targetFieldName) {
					targetField = tf
					break
				}
			}

			if targetField == nil {
				continue
			}

			baseSourceType := p.typeConverter.GetElementType(sourceField.Type)
			baseTargetType := p.typeConverter.GetElementType(targetField.Type)

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
				slog.Debug("Planner: Discovered new dependency rule", "source", newRule.SourceType, "target", newRule.TargetType)
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
func (p *Planner) findSeedRules(typeInfos map[string]*model.TypeInfo, packagePairs []*config.PackagePair) []*config.ConversionRule {
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
				slog.Debug("Planner: Created seed rule", "source", rule.SourceType, "target", rule.TargetType, "direction", rule.Direction)
				seedRules = append(seedRules, rule)
			}
		}
	}
	return seedRules
}

// needsDisambiguation checks if any rules have source and target types with the same base name.
func (p *Planner) needsDisambiguation(rules []*config.ConversionRule) bool {
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
