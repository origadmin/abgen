package config

// Config holds the complete, parsed configuration for a generation task.
type Config struct {
	Version             string
	GenerationContext   GenerationContext
	PackageAliases      map[string]string
	PackagePairs        []*PackagePair
	ConversionRules     []*ConversionRule
	CustomFunctionRules map[string]string
	NamingRules         NamingRules
	GlobalBehaviorRules BehaviorRules
}

// GenerationContext holds information about the package where code is being generated.
type GenerationContext struct {
	PackageName      string
	PackagePath      string
	DirectivePath    string
	MainOutputFile   string
	CustomOutputFile string
}

// PackagePair represents a pairing between a source and a target package.
type PackagePair struct {
	SourcePath string
	TargetPath string
}

// ConversionRule defines a conversion between a source and a target type.
type ConversionRule struct {
	SourceType string
	TargetType string
	Direction  ConversionDirection
	FieldRules FieldRuleSet
	CustomFunc string
}

// NamingRules defines naming conventions for generated types and functions.
type NamingRules struct {
	SourcePrefix string
	SourceSuffix string
	TargetPrefix string
	TargetSuffix string
}

// BehaviorRules defines conversion behaviors.
type BehaviorRules struct {
	GenerateAlias    bool
	DefaultDirection ConversionDirection
}

// FieldRuleSet defines field-specific rules for a given type conversion.
type FieldRuleSet struct {
	Ignore map[string]struct{}
	Remap  map[string]string
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
		GenerationContext:   GenerationContext{},
		PackageAliases:      make(map[string]string),
		PackagePairs:        []*PackagePair{},
		ConversionRules:     []*ConversionRule{},
		CustomFunctionRules: make(map[string]string),
		NamingRules:         NamingRules{},
		GlobalBehaviorRules: BehaviorRules{
			DefaultDirection: DirectionBoth,
		},
	}
}

// Clone creates a deep copy of the Config.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	clone := &Config{
		Version:             c.Version,
		GenerationContext:   c.GenerationContext,
		PackageAliases:      make(map[string]string, len(c.PackageAliases)),
		PackagePairs:        make([]*PackagePair, len(c.PackagePairs)),
		ConversionRules:     make([]*ConversionRule, 0, len(c.ConversionRules)),
		CustomFunctionRules: make(map[string]string, len(c.CustomFunctionRules)),
		NamingRules:         c.NamingRules,
		GlobalBehaviorRules: c.GlobalBehaviorRules,
	}

	for i, pair := range c.PackagePairs {
		if pair != nil {
			pairCopy := *pair
			clone.PackagePairs[i] = &pairCopy
		}
	}

	for _, rule := range c.ConversionRules {
		if rule != nil {
			ruleCopy := &ConversionRule{
				SourceType: rule.SourceType,
				TargetType: rule.TargetType,
				Direction:  rule.Direction,
				CustomFunc: rule.CustomFunc,
				FieldRules: FieldRuleSet{
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
			clone.ConversionRules = append(clone.ConversionRules, ruleCopy)
		}
	}

	for k, v := range c.PackageAliases {
		clone.PackageAliases[k] = v
	}

	for k, v := range c.CustomFunctionRules {
		clone.CustomFunctionRules[k] = v
	}

	return clone
}
