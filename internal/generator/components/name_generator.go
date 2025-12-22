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
// It prioritizes looking up a pre-computed alias from the AliasManager.
func (n *NameGenerator) getCleanBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	// First, check if the AliasManager has a specific name for this exact type.
	// This is the source of truth for any managed type (including slices of named types).
	if alias, ok := n.aliasManager.LookupAlias(info.UniqueKey()); ok {
		return n.capitalize(alias)
	}

	// If no alias exists, fall back to constructing a name based on the type's structure.
	// This handles primitives and unmanaged complex types.
	var baseName string
	switch info.Kind {
	case model.Pointer:
		return n.getCleanBaseName(info.Underlying) // Pointers don't affect the name
	case model.Slice:
		baseName = n.getCleanBaseName(info.Underlying) + "s"
	case model.Array:
		baseName = n.getCleanBaseName(info.Underlying) + "Array"
	case model.Map:
		keyName := n.getCleanBaseName(info.KeyType)
		valName := n.getCleanBaseName(info.Underlying)
		baseName = fmt.Sprintf("%sTo%sMap", keyName, valName)
	case model.Named, model.Struct:
		if info.Name != "" {
			baseName = info.Name
		} else {
			baseName = "Struct" // Anonymous struct
		}
	case model.Primitive:
		baseName = info.Name
	default:
		baseName = "Object"
	}

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
