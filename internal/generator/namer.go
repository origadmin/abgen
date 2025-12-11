package generator

import (
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// Namer handles naming conventions for generated types and functions.
type Namer struct {
	config *config.Config
}

// NewNamer creates a new Namer with the given configuration.
func NewNamer(config *config.Config) *Namer {
	return &Namer{
		config: config,
	}
}

// GetTypeName returns the base type name without any prefixes or suffixes.
// This is used for local type aliases.
func (n *Namer) GetTypeName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	return info.Name
}

// GetFunctionName returns the function name for converting between two types.
// It applies global naming rules (SourcePrefix/Suffix, TargetPrefix/Suffix) to the type names
// used within the function name itself.
func (n *Namer) GetFunctionName(sourceType, targetType *model.TypeInfo) string {
	// According to documentation: Convert + [SourceTypeName] + To + [TargetTypeName]
	sourceName := n.getFunctionTypeNamePart(sourceType, true)
	targetName := n.getFunctionTypeNamePart(targetType, false)

	// For primitive types, use the Title-cased name directly
	if sourceType.Kind == model.Primitive {
		sourceName = strcase.ToCamel(sourceType.Name)
	}
	if targetType.Kind == model.Primitive {
		targetName = strcase.ToCamel(targetType.Name)
	}

	return "Convert" + sourceName + "To" + targetName
}

// getFunctionTypeNamePart returns the type name part used in the function name,
// applying global Source/Target prefixes/suffixes if configured.
func (n *Namer) getFunctionTypeNamePart(info *model.TypeInfo, isSource bool) string {
	if info == nil {
		return ""
	}

	name := info.Name
	if info.Kind == model.Pointer {
		name = strings.TrimPrefix(name, "*")
	}
	if info.Kind == model.Slice {
		name = strings.TrimPrefix(name, "[]")
	}

	if isSource {
		if n.config.NamingRules.SourcePrefix != "" {
			name = n.config.NamingRules.SourcePrefix + name
		}
		if n.config.NamingRules.SourceSuffix != "" {
			name = name + n.config.NamingRules.SourceSuffix
		}
	} else { // is target
		if n.config.NamingRules.TargetPrefix != "" {
			name = n.config.NamingRules.TargetPrefix + name
		}
		if n.config.NamingRules.TargetSuffix != "" {
			name = name + n.config.NamingRules.TargetSuffix
		}
	}
	return name
}
