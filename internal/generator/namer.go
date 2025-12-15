package generator

import (
	"fmt"
	"strings" // Import the strings package

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// Namer is responsible for generating names for functions and type aliases.
type Namer struct {
	config   *config.Config
	aliasMap map[string]string
}

// NewNamer creates a new Namer.
func NewNamer(config *config.Config, aliasMap map[string]string) *Namer {
	return &Namer{
		config:   config,
		aliasMap: aliasMap,
	}
}

// GetFunctionName generates a name for a conversion function.
func (n *Namer) GetFunctionName(source, target *model.TypeInfo) string {
	sourceName := n.getAliasedOrBaseName(source)
	targetName := n.getAliasedOrBaseName(target)
	return fmt.Sprintf("Convert%sTo%s", sourceName, targetName)
}

// GetPrimitiveConversionStubName generates a name for a primitive conversion stub function,
// incorporating parent type and field names for better context.
// The naming rule is: Convert + ParentSourceTypeAlias + SourceFieldName + To + ParentTargetTypeAlias + TargetFieldName
func (n *Namer) GetPrimitiveConversionStubName(
	parentSource *model.TypeInfo, sourceField *model.FieldInfo,
	parentTarget *model.TypeInfo, targetField *model.FieldInfo,
) string {
	parentSourceName := n.getAliasedOrBaseName(parentSource)
	parentTargetName := n.getAliasedOrBaseName(parentTarget)

	sourceFieldName := strings.Title(sourceField.Name)
	targetFieldName := strings.Title(targetField.Name)

	return fmt.Sprintf("Convert%s%sTo%s%s",
		parentSourceName, sourceFieldName,
		parentTargetName, targetFieldName,
	)
}

// GetAlias generates a type alias based on the naming rules.
func (n *Namer) GetAlias(info *model.TypeInfo, isSource, disambiguate bool) string {
	// If it's a primitive type, we don't apply any prefixes/suffixes from naming rules.
	if info.Kind == model.Primitive {
		return info.Name
	}

	baseName := info.Name
	var prefix, suffix string

	// Determine if any specific rule is set for either source or target.
	anySourceRule := n.config.NamingRules.SourcePrefix != "" || n.config.NamingRules.SourceSuffix != ""
	anyTargetRule := n.config.NamingRules.TargetPrefix != "" || n.config.NamingRules.TargetSuffix != ""
	anySpecificRule := anySourceRule || anyTargetRule

	if isSource {
		prefix = n.config.NamingRules.SourcePrefix
		suffix = n.config.NamingRules.SourceSuffix
		// If there are NO specific rules AT ALL, and we need to disambiguate, add "Source".
		if !anySpecificRule && disambiguate {
			suffix = "Source"
		}
	} else { // is Target
		prefix = n.config.NamingRules.TargetPrefix
		suffix = n.config.NamingRules.TargetSuffix
		// If there are NO specific rules AT ALL, and we need to disambiguate, add "Target".
		if !anySpecificRule && disambiguate {
			suffix = "Target"
		}
	}

	return prefix + baseName + suffix
}

// getAliasedOrBaseName returns the alias if it exists, otherwise returns the base name.
// For primitive types, it returns the capitalized primitive name.
func (n *Namer) getAliasedOrBaseName(info *model.TypeInfo) string {
	if info.Kind == model.Primitive {
		return strings.Title(info.Name) // Capitalize primitive names
	}

	fqn := info.FQN()
	if fqn == "" {
		// This case should ideally not be hit for non-primitive types if TypeInfo is well-formed.
		// For safety, return capitalized name if it's a primitive that somehow slipped through.
		if info.Kind == model.Primitive {
			return strings.Title(info.Name)
		}
		return info.Name // Fallback for unnamed non-primitives
	}
	if alias, ok := n.aliasMap[fqn]; ok {
		return alias
	}
	return info.Name
}
