package components

import (
	"fmt"
	"unicode"

	"github.com/origadmin/abgen/internal/model"
)

// NameGenerator implements the model.NameGenerator interface.
// It relies on an AliasManager to get the definitive, role-based name for a type.
type NameGenerator struct {
	aliasManager model.AliasManager
}

// NewNameGenerator creates a new name generator that depends on an alias manager.
func NewNameGenerator(aliasManager model.AliasManager) model.NameGenerator {
	return &NameGenerator{
		aliasManager: aliasManager,
	}
}

// ConversionFunctionName returns a standardized name for a function that converts between two types.
func (n *NameGenerator) ConversionFunctionName(source, target *model.TypeInfo) string {
	sourceName := n.getCleanBaseName(source)
	targetName := n.getCleanBaseName(target)
	return fmt.Sprintf("Convert%sTo%s", sourceName, targetName)
}

// FieldConversionFunctionName returns a standardized name for a function that converts a specific field.
func (n *NameGenerator) FieldConversionFunctionName(sourceParent, targetParent *model.TypeInfo, sourceField, targetField *model.FieldInfo) string {
	sourceParentName := n.getCleanBaseName(sourceParent)
	targetParentName := n.getCleanBaseName(targetParent)
	fieldName := n.capitalize(sourceField.Name)
	return fmt.Sprintf("Convert%s%sTo%s%s", sourceParentName, fieldName, targetParentName, fieldName)
}

// getCleanBaseName recursively finds the base name for a type, suitable for use in a function name.
// For named types, it defers to the AliasManager to get the correct, role-aware name.
func (n *NameGenerator) getCleanBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	var baseName string
	switch info.Kind {
	case model.Pointer:
		return n.getCleanBaseName(info.Underlying)
	case model.Slice:
		// Suffix 's' is used for slice types as per existing test cases (e.g., User -> Users)
		baseName = n.getCleanBaseName(info.Underlying) + "s"
	case model.Array:
		baseName = n.getCleanBaseName(info.Underlying) + "Array"
	case model.Map:
		keyName := n.getCleanBaseName(info.KeyType)
		valName := n.getCleanBaseName(info.Underlying)
		baseName = fmt.Sprintf("%sTo%sMap", keyName, valName)
	case model.Named, model.Struct:
		// CRITICAL: Use the alias manager as the source of truth for the type's name.
		// The alias manager is responsible for applying all naming rules (suffixes, etc.).
		if alias, ok := n.aliasManager.LookupAlias(info.UniqueKey()); ok {
			baseName = alias
		} else if info.Name != "" {
			// Fallback for types not managed by the alias manager (e.g., time.Time, or primitives).
			baseName = info.Name
		} else {
			baseName = "Struct" // Anonymous struct
		}
	case model.Primitive:
		baseName = info.Name
	default:
		baseName = "Object"
	}

	// Only capitalize the final computed base name.
	return n.capitalize(baseName)
}

// capitalize capitalizes the first letter of a string.
func (n *NameGenerator) capitalize(s string) string {
	if s == "" {
		return ""
	}
	// This logic is safe for already capitalized strings.
	if r := rune(s[0]); unicode.IsUpper(r) {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
