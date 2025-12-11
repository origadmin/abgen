// Package config defines the data structures for abgen's configuration and rules.
package config

// Global constants for the application.
const (
	Application = "abgen"
	Description = "Auto generate conversion code between structs"
	WebSite     = "https://github.com/origadmin/abgen"
	UI          = "abgen"
)

// Config holds the complete, parsed configuration for a generation task.
// It is designed to be stateless and serializable.
type Config struct {
	// GenerationContext holds information about the target package for generation.
	GenerationContext GenerationContext
	// PackageAliases maps a package alias to its full import path. It enforces a one-to-one relationship.
	PackageAliases map[string]string
	// PackagePairs defines the source-to-target package mappings.
	PackagePairs []*PackagePair
	// ConversionRules defines the type-level conversion rules.
	ConversionRules []*ConversionRule
	// NamingRules defines how to name generated types and functions.
	NamingRules NamingRule
	// GlobalBehaviorRules defines global conversion behaviors.
	GlobalBehaviorRules BehaviorRule
}

// GenerationContext holds information about the package where code is being generated.
type GenerationContext struct {
	PackageName string // e.g., "users"
	PackagePath string // e.g., "github.com/my/project/users"
}

// PackagePair represents a pairing between a source and a target package.
type PackagePair struct {
	SourcePath string
	TargetPath string
}

// ConversionRule defines a conversion between a source and a target type.
type ConversionRule struct {
	SourceType string // Fully-qualified type name
	TargetType string // Fully-qualified type name
	Direction  ConversionDirection
	FieldRules FieldRuleSet
}

// NamingRule defines naming conventions for generated types and functions.
type NamingRule struct {
	SourcePrefix string
	SourceSuffix string
	TargetPrefix string
	TargetSuffix string
}

// BehaviorRule defines conversion behaviors.
type BehaviorRule struct {
	GenerateAlias bool
}

// FieldRuleSet defines field-specific rules for a given type conversion.
type FieldRuleSet struct {
	Ignore map[string]struct{}      // Fields to ignore
	Remap  map[string]string        // Fields to remap (source -> target)
}

// ConversionDirection represents the direction of conversion.
type ConversionDirection string

const (
	DirectionBoth   ConversionDirection = "both"
	DirectionOneway ConversionDirection = "oneway"
)

// NewConfig creates a new, empty configuration object.
func NewConfig() *Config {
	return &Config{
		GenerationContext: GenerationContext{},
		PackageAliases:    make(map[string]string),
		PackagePairs:      []*PackagePair{},
		ConversionRules:   []*ConversionRule{},
		NamingRules: NamingRule{
			SourcePrefix: "",
			SourceSuffix: "",
			TargetPrefix: "",
			TargetSuffix: "",
		},
		GlobalBehaviorRules: BehaviorRule{
			GenerateAlias: false,
		},
	}
}
