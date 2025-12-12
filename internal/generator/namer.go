package generator

import (
	"github.com/iancoleman/strcase"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// Namer handles naming conventions for generated types and functions.
type Namer struct {
	config   *config.Config
	aliasMap map[string]string // Reference to the aliasMap in Generator
}

// NewNamer creates a new Namer with the given configuration.
func NewNamer(config *config.Config, aliasMap map[string]string) *Namer {
	return &Namer{
		config:   config,
		aliasMap: aliasMap,
	}
}

// GetTypeName returns the base type name or its generated alias for local usage.
func (n *Namer) GetTypeName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	// Prioritize alias if it exists
	if alias, ok := n.aliasMap[info.FQN()]; ok {
		return alias
	}
	// Fallback to original name if no alias
	return info.Name
}

// GetFunctionName returns the function name for converting between two types.
func (n *Namer) GetFunctionName(sourceInfo, targetInfo *model.TypeInfo) string {
	// Use the aliases for the function name parts
	sourceName := n.GetTypeName(sourceInfo)
	targetName := n.GetTypeName(targetInfo)

	// Ensure primitives are camel cased if not already (e.g. "int" -> "Int")
	if sourceInfo.Kind == model.Primitive {
		sourceName = strcase.ToCamel(sourceName)
	}
	if targetInfo.Kind == model.Primitive {
		targetName = strcase.ToCamel(targetName)
	}

	return "Convert" + sourceName + "To" + targetName
}
