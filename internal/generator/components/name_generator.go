package components

import (
	"fmt"
	"strings"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// NameGenerator implements the model.NameGenerator interface.
type NameGenerator struct {
	importManager model.ImportManager
	config        *config.Config
}

// NewNameGenerator creates a new name generator.
func NewNameGenerator(cfg *config.Config, importManager model.ImportManager) model.NameGenerator {
	return &NameGenerator{
		config:        cfg,
		importManager: importManager,
	}
}

// TypeName returns the correct Go syntax for a given type, including its package qualifier.
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
	if info == nil {
		sb.WriteString("interface{}")
		return
	}

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
	case model.Named, model.Struct:
		if info.Name != "" {
			if info.ImportPath != "" {
				alias := n.importManager.Add(info.ImportPath)
				if alias != "" {
					sb.WriteString(alias)
					sb.WriteString(".")
				}
			}
			sb.WriteString(info.Name)
		} else {
			sb.WriteString("struct{}")
		}
	case model.Primitive:
		sb.WriteString(info.Name)
	case model.Interface:
		sb.WriteString("interface{}")
	default:
		sb.WriteString("interface{}")
	}
}

// ConversionFunctionName returns a standardized name for a conversion function.
func (n *NameGenerator) ConversionFunctionName(source, target *model.TypeInfo) string {
	sourceName := n.getCleanBaseName(source)
	targetName := n.getCleanBaseName(target)
	return fmt.Sprintf("Convert%sTo%s", sourceName, targetName)
}

// getCleanBaseName recursively finds the base name of a type for use in function names.
func (n *NameGenerator) getCleanBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	switch info.Kind {
	case model.Pointer:
		return n.getCleanBaseName(info.Underlying)
	case model.Slice:
		return n.getCleanBaseName(info.Underlying) + "s"
	case model.Array:
		return n.getCleanBaseName(info.Underlying) + "Array"
	case model.Map:
		keyName := n.getCleanBaseName(info.KeyType)
		valName := n.getCleanBaseName(info.Underlying)
		return fmt.Sprintf("%sTo%sMap", keyName, valName)
	case model.Named, model.Struct:
		if info.Name != "" {
			return info.Name
		}
		return "Struct"
	case model.Primitive:
		return info.Name
	default:
		return "Object"
	}
}
