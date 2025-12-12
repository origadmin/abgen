package config

import (
	"strings"
)

// Global constants for the application.
const (
	Application = "abgen"
	Description = "Auto generate conversion code between structs"
	WebSite     = "https://github.com/origadmin/abgen"
	UI          = "abgen"
)

// Config holds the complete, parsed configuration for a generation task.
type Config struct {
	GenerationContext   GenerationContext
	PackageAliases      map[string]string
	LocalAliases        map[string]string
	PackagePairs        []*PackagePair
	ConversionRules     []*ConversionRule
	CustomFunctionRules map[string]string // New: Stores sourceFQN->targetFQN to funcName mappings
	NamingRules         NamingRule
	GlobalBehaviorRules BehaviorRule
}

// GenerationContext holds information about the package where code is being generated.
type GenerationContext struct {
	PackageName   string // e.g., "users"
	PackagePath   string // e.g., "github.com/my/project/users"
	DirectivePath string // The path to the directory containing the directive file
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
	CustomFunc string // The name of the custom conversion function to use.
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
	Ignore map[string]struct{} // Fields to ignore
	Remap  map[string]string   // Fields to remap (source -> target)
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
		LocalAliases:        make(map[string]string),
		PackagePairs:        []*PackagePair{},
		ConversionRules:     []*ConversionRule{},
		CustomFunctionRules: make(map[string]string), // Initialize the new map
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

// RequiredPackages gathers all unique package paths from the configuration.
func (c *Config) RequiredPackages() []string {
	pathMap := make(map[string]struct{})

	if c.GenerationContext.PackagePath != "" {
		pathMap[c.GenerationContext.PackagePath] = struct{}{}
	}
	for _, path := range c.PackageAliases {
		pathMap[path] = struct{}{}
	}
	for _, pair := range c.PackagePairs {
		pathMap[pair.SourcePath] = struct{}{}
		pathMap[pair.TargetPath] = struct{}{}
	}
	for _, rule := range c.ConversionRules {
		if pkgPath := getPkgPath(rule.SourceType); pkgPath != "" {
			pathMap[pkgPath] = struct{}{}
		}
		if pkgPath := getPkgPath(rule.TargetType); pkgPath != "" {
			pathMap[pkgPath] = struct{}{}
		}
	}
	// Also include packages from custom function rules
	for key := range c.CustomFunctionRules {
		parts := strings.Split(key, "->")
		if len(parts) == 2 {
			if pkgPath := getPkgPath(parts[0]); pkgPath != "" {
				pathMap[pkgPath] = struct{}{}
			}
			if pkgPath := getPkgPath(parts[1]); pkgPath != "" {
				pathMap[pkgPath] = struct{}{}
			}
		}
	}

	paths := make([]string, 0, len(pathMap))
	for path := range pathMap {
		paths = append(paths, path)
	}
	return paths
}

// RequiredTypeFQNs gathers all unique fully-qualified type names (FQNs).
func (c *Config) RequiredTypeFQNs() []string {
	fqnMap := make(map[string]struct{})

	for _, rule := range c.ConversionRules {
		if rule.SourceType != "" {
			fqnMap[rule.SourceType] = struct{}{}
		}
		if rule.TargetType != "" {
			fqnMap[rule.TargetType] = struct{}{}
		}
	}
	for key := range c.CustomFunctionRules {
		parts := strings.Split(key, "->")
		if len(parts) == 2 {
			if parts[0] != "" {
				fqnMap[parts[0]] = struct{}{}
			}
			if parts[1] != "" {
				fqnMap[parts[1]] = struct{}{}
			}
		}
	}

	fqns := make([]string, 0, len(fqnMap))
	for fqn := range fqnMap {
		fqns = append(fqns, fqn)
	}
	return fqns
}

// getPkgPath extracts the package path from a fully-qualified type name.
func getPkgPath(fqn string) string {
	lastDot := strings.LastIndex(fqn, ".")
	if lastDot == -1 {
		return ""
	}
	return fqn[:lastDot]
}
