package components

import (
	"fmt"
	"unicode"

	"github.com/origadmin/abgen/internal/model"
)

// NameGenerator implements the model.NameGenerator interface.
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

// getCleanBaseName finds the authoritative name for a type.
// It prioritizes looking up a pre-computed alias from the AliasManager.
// If no alias is found, it constructs a name from the type's structure.
func (n *NameGenerator) getCleanBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	// The AliasManager is the source of truth for all managed types.
	if alias, ok := n.aliasManager.LookupAlias(info.UniqueKey()); ok {
		return n.capitalize(alias)
	}

	// Fallback for unmanaged types (e.g., primitives, time.Time, etc.)
	var baseName string
	switch info.Kind {
	case model.Pointer:
		return n.getCleanBaseName(info.Underlying)
	case model.Slice:
		elemName := n.getCleanBaseName(info.Underlying)
		baseName = elemName + "s"
	case model.Array:
		baseName = n.getCleanBaseName(info.Underlying) + "Array"
	case model.Map:
		keyName := n.getCleanBaseName(info.KeyType)
		valName := n.getCleanBaseName(info.Underlying)
		baseName = fmt.Sprintf("%sTo%sMap", keyName, valName)
	case model.Named, model.Struct:
		baseName = info.Name
	case model.Primitive:
		baseName = info.Name
	default:
		baseName = "Object"
	}

	return n.capitalize(baseName)
}

func (n *NameGenerator) capitalize(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
