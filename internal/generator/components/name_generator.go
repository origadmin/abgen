package components

import (
	"fmt"
	"unicode"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// NameGenerator implements the model.NameGenerator interface.
// Its sole responsibility is to generate names for functions and variables based on TypeInfo.
type NameGenerator struct {
	config *config.Config
}

// NewNameGenerator creates a new name generator.
func NewNameGenerator(cfg *config.Config) model.NameGenerator {
	return &NameGenerator{
		config: cfg,
	}
}

// ConversionFunctionName returns a standardized name for a function that converts between two types.
func (n *NameGenerator) ConversionFunctionName(source, target *model.TypeInfo) string {
	sourceName := n.getCleanBaseName(source)
	targetName := n.getCleanBaseName(target)

	// Apply suffixes from config, but only for non-primitive types
	if n.config.NamingRules.SourceSuffix != "" && n.shouldApplySuffix(source) {
		sourceName += n.capitalize(n.config.NamingRules.SourceSuffix)
	}
	if n.config.NamingRules.TargetSuffix != "" && n.shouldApplySuffix(target) {
		targetName += n.capitalize(n.config.NamingRules.TargetSuffix)
	}

	return fmt.Sprintf("Convert%sTo%s", sourceName, targetName)
}

// FieldConversionFunctionName returns a standardized name for a function that converts a specific field
// between two parent structs. The name is more specific, like Convert<SourceStruct><Field>To<TargetStruct><Field>.
func (n *NameGenerator) FieldConversionFunctionName(sourceParent, targetParent *model.TypeInfo, sourceField, targetField *model.FieldInfo) string {
	sourceParentName := n.getCleanBaseName(sourceParent)
	targetParentName := n.getCleanBaseName(targetParent)
	fieldName := n.capitalize(sourceField.Name) // Use the source field name as the base

	// Apply parent suffixes
	if n.config.NamingRules.SourceSuffix != "" {
		sourceParentName += n.capitalize(n.config.NamingRules.SourceSuffix)
	}
	if n.config.NamingRules.TargetSuffix != "" {
		targetParentName += n.capitalize(n.config.NamingRules.TargetSuffix)
	}

	return fmt.Sprintf("Convert%s%sTo%s%s", sourceParentName, fieldName, targetParentName, fieldName)
}

// shouldApplySuffix determines if a suffix should be applied to the type name.
// Generally, we only apply suffixes to named types (structs, custom types),
// not to primitives like int, string, time.Time, etc.
func (n *NameGenerator) shouldApplySuffix(info *model.TypeInfo) bool {
	if info == nil {
		return false
	}
	switch info.Kind {
	case model.Primitive:
		return false
	case model.Named, model.Struct:
		// Even if it's a named type, we might want to exclude standard library types
		// if they are treated as "primitives" in the domain (like time.Time).
		// However, model.Primitive usually covers basic Go types.
		// time.Time is often model.Struct or model.Named depending on how it's loaded,
		// but usually we want to treat it as a primitive-like type for naming.
		if info.ImportPath == "time" && info.Name == "Time" {
			return false
		}
		return true
	case model.Pointer, model.Slice, model.Array:
		return n.shouldApplySuffix(info.Underlying)
	case model.Map:
		return n.shouldApplySuffix(info.KeyType) || n.shouldApplySuffix(info.Underlying)
	default:
		return false
	}
}

// getCleanBaseName recursively finds and sanitizes the base name of a type for use in function names.
// It produces a capitalized, CamelCase name suitable for embedding in another identifier.
func (n *NameGenerator) getCleanBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	var baseName string
	switch info.Kind {
	case model.Pointer:
		// Pointers don't typically affect the base name for conversion functions.
		return n.getCleanBaseName(info.Underlying)
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

func (n *NameGenerator) capitalize(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
