// Package types implements the functions, types, and interfaces for the module.
package types

// EndpointConfig describes a source or target endpoint for a conversion.
type EndpointConfig struct {
	Type       string // Fully qualified type name, e.g., "github.com/my/pkg.UserEntity"
	LocalAlias string // Local type alias name if applicable
	Prefix     string // Prefix to add to the type name for function naming.
	Suffix     string // Suffix to add to the type name, e.g., "Ent".
}

// Clone returns a deep copy of the EndpointConfig.
func (ec *EndpointConfig) Clone() *EndpointConfig {
	if ec == nil {
		return nil
	}
	return &EndpointConfig{
		Type:       ec.Type,
		LocalAlias: ec.LocalAlias,
		Prefix:     ec.Prefix,
		Suffix:     ec.Suffix,
	}
}

// TypeConversionRule defines a rule for converting between two specific types.
type TypeConversionRule struct {
	SourceTypeName string // The name of the source type (e.g., "Gender", "time.Time").
	TargetTypeName string // The name of the target type (e.g., "string", "timestamp.Timestamp").
	ConvertFunc    string // The name of the custom conversion function to use.
}

// ConversionConfig holds the complete configuration for a single conversion pair.
// It also holds global default configuration rules that can be overridden by specific directives
// associated with this conversion.
type ConversionConfig struct {
	Source              *EndpointConfig
	Target              *EndpointConfig
	Direction           string // "both", "to", "from"
	IgnoreTypes         map[string]bool // Global/Package level ignored types, copied or merged into this specific config
	IgnoreFields        map[string]bool
	RemapFields         map[string]string // Maps TargetField -> SourcePath (e.g., "RoleIDs": "Edges.Roles.ID")
	TypeConversionRules []TypeConversionRule
	GeneratorPkgPath    string // The package path where the generation is happening

	// Global/Package level settings, copied or merged into this specific config
	// These will be inherited from the "default" config if not explicitly set for this conversion.
	SourcePackage       string
	TargetPackage       string
	SourcePrefix        string
	SourceSuffix        string
	TargetPrefix        string
	TargetSuffix        string
}

// Clone returns a deep copy of the ConversionConfig.
func (cc *ConversionConfig) Clone() *ConversionConfig {
	if cc == nil {
		return nil
	}
	clone := &ConversionConfig{
		Source:              cc.Source.Clone(),
		Target:              cc.Target.Clone(),
		Direction:           cc.Direction,
		GeneratorPkgPath:    cc.GeneratorPkgPath,
		SourcePackage:       cc.SourcePackage,
		TargetPackage:       cc.TargetPackage,
		SourcePrefix:        cc.SourcePrefix,
		SourceSuffix:        cc.SourceSuffix,
		TargetPrefix:        cc.TargetPrefix,
		TargetSuffix:        cc.TargetSuffix,
	}

	// Deep copy maps
	if cc.IgnoreTypes != nil {
		clone.IgnoreTypes = make(map[string]bool, len(cc.IgnoreTypes))
		for k, v := range cc.IgnoreTypes {
			clone.IgnoreTypes[k] = v
		}
	}
	if cc.IgnoreFields != nil {
		clone.IgnoreFields = make(map[string]bool, len(cc.IgnoreFields))
		for k, v := range cc.IgnoreFields {
			clone.IgnoreFields[k] = v
		}
	}
	if cc.RemapFields != nil {
		clone.RemapFields = make(map[string]string, len(cc.RemapFields))
		for k, v := range cc.RemapFields {
			clone.RemapFields[k] = v
		}
	}
	if cc.TypeConversionRules != nil {
		clone.TypeConversionRules = make([]TypeConversionRule, len(cc.TypeConversionRules))
		copy(clone.TypeConversionRules, cc.TypeConversionRules)
	}
	return clone
}