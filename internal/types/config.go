package types

// TypeEndpoint represents one side of a specific type pairing.
type TypeEndpoint struct {
	Type      string // Fully qualified type name, e.g., "github.com/my/pkg.UserEntity"
	AliasType string // The local alias for the type, if one exists.
	// Add a field to store the resolved TypeInfo for this endpoint
	TypeInfo *TypeInfo
}

// TypePair defines a single, pure pairing of a source type to a target type.
// It contains no rules, only the identity of the two endpoints.
type TypePair struct {
	Source *TypeEndpoint
	Target *TypeEndpoint
}

// PackageEndpoint defines the global rules for a source or target package.
type PackageEndpoint struct {
	Package string // The package path, used for package-level pairing.
	Prefix  string // Global prefix for this endpoint.
	Suffix  string // Global suffix for this endpoint.
}

// CustomRule defines a rule for converting between two specific types using a custom function.
type CustomRule struct {
	SourceTypeName string
	TargetTypeName string
	ConvertFunc    string
}

// ConversionConfig holds the single, unified configuration for the entire package generation.
type ConversionConfig struct {
	// Source and Target define the global package-level rules.
	Source *PackageEndpoint
	Target *PackageEndpoint

	// Pairs holds all the specific `source` to `target` pairings defined via directives.
	Pairs []*TypePair

	// Direction applies to all pairs.
	Direction string // "both", "oneway"

	// Global maps for field-level and type-level control.
	IgnoreFields   map[string]bool
	RemapFields    map[string]string // Maps TargetField -> SourcePath
	IgnoreTypes    map[string]bool
	CustomRules    []CustomRule

	// ContextPackagePath is the package path where the directives are being processed.
	// It's a necessary parameter to understand the current context.
	ContextPackagePath string
}

// NewDefaultConfig creates a new ConversionConfig with initialized maps and default values.
func NewDefaultConfig() *ConversionConfig {
	return &ConversionConfig{
		Source:       &PackageEndpoint{},
		Target:       &PackageEndpoint{},
		Pairs:        make([]*TypePair, 0),
		Direction:    "oneway",
		IgnoreFields: make(map[string]bool),
		RemapFields:  make(map[string]string),
		IgnoreTypes:  make(map[string]bool),
		CustomRules:  make([]CustomRule, 0),
	}
}

// IsPrimitiveType checks if a given type name is one of Go's built-in primitive types.
func IsPrimitiveType(name string) bool {
	switch name {
	case "bool", "byte", "rune", "string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128", "error":
		return true
	default:
		return false
	}
}
