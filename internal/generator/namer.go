package generator

import (
	"fmt"
	"strings" // Import the strings package

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

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

func (n *Namer) getFinalAlias(baseName string, isSource, disambiguate bool) string {
	alias := baseName // baseName is already Title-cased (e.g., "Item", "Order", "Items")
	var prefix, suffix string

	if isSource {
		prefix = n.config.NamingRules.SourcePrefix
		suffix = n.config.NamingRules.SourceSuffix
	} else { // is Target
		prefix = n.config.NamingRules.TargetPrefix
		suffix = n.config.NamingRules.TargetSuffix
	}

	// Apply configured prefix and suffix
	alias = prefix + alias + suffix

	// If disambiguation is needed and not already handled by a config suffix/prefix,
	// apply default disambiguation suffix.
	if disambiguate {
		if isSource && n.config.NamingRules.SourceSuffix == "" && n.config.NamingRules.SourcePrefix == "" {
			alias += "Source"
		} else if !isSource && n.config.NamingRules.TargetSuffix == "" && n.config.NamingRules.TargetPrefix == "" {
			alias += "Target"
		}
	}
	return alias
}

// GetAlias generates a suitable alias for a given TypeInfo.
func (n *Namer) GetAlias(info *model.TypeInfo, isSource, disambiguate bool) string {
	if info.Kind == model.Primitive {
		return info.Name
	}

	var baseName string
	if info.Kind == model.Slice && info.Underlying != nil {
		// For slices, take the base name of the underlying element, pluralize it.
		baseName = cases.Title(language.English).String(info.Underlying.Name) + "s" // e.g., "Items"
	} else {
		// For named types, just take its name.
		baseName = cases.Title(language.English).String(info.Name) // e.g., "Item", "Order"
	}

	return n.getFinalAlias(baseName, isSource, disambiguate)
}
// getAliasedOrBaseName returns the alias if it exists, otherwise returns the base name.
// For primitive types, it returns the capitalized primitive name.
func (n *Namer) getAliasedOrBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	// For primitives, just return the name as is. Capitalization will be handled by GetFunctionName.
	if info.Kind == model.Primitive {
		return info.Name
	}

	key := info.UniqueKey() // Use UniqueKey() for all types, including slices

	if alias, ok := n.aliasMap[key]; ok {
		return alias
	}
	return info.Name // Fallback to base name if no alias found
}
