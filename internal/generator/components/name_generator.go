package components

import (
	"fmt"
	"strings"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// NameGenerator implements the model.NameGenerator interface.
// It relies on an ImportManager to get the correct package qualifiers for types.
type NameGenerator struct {
	importManager model.ImportManager
	config        *config.Config // Keep config for naming rules if needed in the future
}

// NewNameGenerator creates a new name generator.
// It requires an ImportManager to resolve package aliases correctly.
func NewNameGenerator(cfg *config.Config, importManager model.ImportManager) model.NameGenerator {
	return &NameGenerator{
		config:        cfg,
		importManager: importManager,
	}
}

// TypeName returns the correct Go syntax for a given type, including its package qualifier.
// It handles primitives, named types, pointers, slices, arrays, and maps.
func (n *NameGenerator) TypeName(info *model.TypeInfo) string {
	if info == nil {
		return "nil"
	}

	var sb strings.Builder
	n.buildTypeName(&sb, info)
	return sb.String()
}

// buildTypeName is a recursive helper to construct the type string.
func (n *NameGenerator) buildTypeName(sb *strings.Builder, info *model.TypeInfo) {
	switch info.Kind {
	case model.Pointer:
		sb.WriteString("*")
		n.buildTypeName(sb, info.Underlying)
	case model.Slice:
		sb.WriteString("[]")
		n.buildTypeName(sb, info.Underlying)
	case model.Array:
		sb.WriteString(fmt.Sprintf("[%d]", info.ArrayLen))
		n.buildTypeName(sb, info.Underlying)
	case model.Map:
		sb.WriteString("map[")
		n.buildTypeName(sb, info.KeyType)
		sb.WriteString("]")
		n.buildTypeName(sb, info.Underlying)
	case model.Named:
		// For named types, get the package alias from the import manager.
		if info.ImportPath != "" {
			alias := n.importManager.Add(info.ImportPath)
			if alias != "" {
				sb.WriteString(alias)
				sb.WriteString(".")
			}
		}
		sb.WriteString(info.Name)
	case model.Primitive:
		sb.WriteString(info.Name)
	case model.Interface:
		// Assuming generic interface for now.
		sb.WriteString("interface{}")
	default:
		// Fallback for unknown or unhandled types.
		sb.WriteString("interface{}")
	}
}

// ConversionFunctionName returns a standardized name for a function that converts from a source to a target type.
// It creates a name like 'ConvertUserSourceToUserTarget'.
func (n *NameGenerator) ConversionFunctionName(source, target *model.TypeInfo) string {
	// To create a clean function name, we use the type's base name without package qualifiers.
	// We also handle composite types to make the name readable.
	sourceName := n.getCleanBaseName(source)
	targetName := n.getCleanBaseName(target)
	return fmt.Sprintf("Convert%sTo%s", sourceName, targetName)
}

// getCleanBaseName recursively finds the base name of a type and makes it suitable for a function name.
func (n *NameGenerator) getCleanBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	switch info.Kind {
	case model.Pointer:
		return n.getCleanBaseName(info.Underlying) // Pointers don't change the name
	case model.Slice:
		return n.getCleanBaseName(info.Underlying) + "s" // e.g., User -> Users
	case model.Array:
		return n.getCleanBaseName(info.Underlying) + "Array" // e.g., User -> UserArray
	case model.Map:
		keyName := n.getCleanBaseName(info.KeyType)
		valName := n.getCleanBaseName(info.Underlying)
		return fmt.Sprintf("%sTo%sMap", keyName, valName) // e.g., StringToIntMap
	case model.Named, model.Primitive:
		return info.Name
	default:
		return "Object"
	}
}
