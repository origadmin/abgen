// Package config defines the rule models for the abgen system.
// It contains the data structures that hold parsed directive information.
package config

// GenerationContext holds information about the target package for generation.
type GenerationContext struct {
	// PackageName is the name of the package for the generated file (e.g., "users").
	PackageName string
	// PackagePath is the import path of the package (e.g., "github.com/my/project/users").
	PackagePath string
}

// RuleSet holds all the rules parsed from directives.
type RuleSet struct {
	// Context holds information about the package where code is being generated.
	Context GenerationContext

	// PackagePairs maps source package paths to target package paths.
	PackagePairs map[string]string

	// NamingRules defines how to name types and functions.
	NamingRules NamingRuleSet

	// BehaviorRules defines conversion behaviors.
	BehaviorRules BehaviorRuleSet

	// FieldRules defines field-specific rules.
	FieldRules FieldRuleSet
}

// NewRuleSet creates a new empty RuleSet.
func NewRuleSet() *RuleSet {
	return &RuleSet{
		Context:      GenerationContext{},
		PackagePairs: make(map[string]string),
		NamingRules: NamingRuleSet{
			SourcePrefix: "",
			SourceSuffix: "",
			TargetPrefix: "",
			TargetSuffix: "",
		},
		BehaviorRules: BehaviorRuleSet{
			GenerateAlias: false,
			Direction:     make(map[string]string),
		},
		FieldRules: FieldRuleSet{
			Ignore: make(map[string]map[string]struct{}),
			Remap:  make(map[string]map[string]string),
		},
	}
}

// NamingRuleSet defines naming conventions for generated types and functions.
type NamingRuleSet struct {
	// SourcePrefix is the prefix to add to source type names.
	SourcePrefix string
	// SourceSuffix is the suffix to add to source type names.
	SourceSuffix string
	// TargetPrefix is the prefix to add to target type names.
	TargetPrefix string
	// TargetSuffix is the suffix to add to target type names.
	TargetSuffix string
}

// BehaviorRuleSet defines conversion behaviors.
type BehaviorRuleSet struct {
	// GenerateAlias indicates whether to generate type aliases.
	GenerateAlias bool
	// Direction maps type pairs to conversion direction ("to", "from", "both").
	Direction map[string]string
}

// FieldRuleSet defines field-specific rules.
type FieldRuleSet struct {
	// Ignore maps type names to sets of field names to ignore.
	Ignore map[string]map[string]struct{}
	// Remap maps type names to field remapping rules (source field -> target field).
	Remap map[string]map[string]string
}

// ConversionDirection represents the direction of conversion.
type ConversionDirection string

const (
	DirectionTo   ConversionDirection = "to"
	DirectionFrom ConversionDirection = "from"
	DirectionBoth ConversionDirection = "both"
)