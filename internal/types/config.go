// Package types implements the functions, types, and interfaces for the module.
package types

// EndpointConfig describes a source or target endpoint for a conversion.
type EndpointConfig struct {
	Type   string // Fully qualified type name, e.g., "github.com/my/pkg.UserEntity"
	Prefix string // Prefix to add to the type name for function naming.
	Suffix string // Suffix to add to the type name, e.g., "Ent".
}

// TypeConversionRule defines a rule for converting between two specific types.
type TypeConversionRule struct {
	SourceTypeName string // The name of the source type (e.g., "Gender", "time.Time").
	TargetTypeName string // The name of the target type (e.g., "string", "timestamp.Timestamp").
	ConvertFunc    string // The name of the custom conversion function to use.
}

// ConversionConfig holds the complete configuration for a single conversion pair.
type ConversionConfig struct {
	Source              *EndpointConfig
	Target              *EndpointConfig
	Direction           string // "both", "to", "from"
	IgnoreFields        map[string]bool
	RemapFields         map[string]string // Maps TargetField -> SourcePath (e.g., "RoleIDs": "Edges.Roles.ID")
	TypeConversionRules []TypeConversionRule
	GeneratorPkgPath    string // The package path where the generation is happening
}

// PackageConversionConfig holds the configuration for converting all matching types between two packages.
type PackageConversionConfig struct {
	SourcePackage       string
	TargetPackage       string
	Direction           string
	IgnoreTypes         map[string]bool
	IgnoreFields        map[string]bool
	RemapFields         map[string]string
	TypeConversionRules []TypeConversionRule
	SourcePrefix        string
	SourceSuffix        string
	TargetPrefix        string
	TargetSuffix        string
}

// ConversionNode represents a type in the conversion graph.
type ConversionNode struct {
	FromConversions []string // List of types that can be converted TO this type.
	ToConversions   []string // List of types this type can be converted FROM.
	Configs         map[string]*ConversionConfig
}

// ConversionGraph represents the entire map of type conversions.
type ConversionGraph map[string]*ConversionNode