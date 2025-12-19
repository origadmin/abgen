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

// ConversionFunctionName returns a standardized name for a conversion function.
func (n *NameGenerator) ConversionFunctionName(source, target *model.TypeInfo) string {
	sourceName := n.getCleanBaseName(source)
	targetName := n.getCleanBaseName(target)

	// Apply suffixes from config
	if n.config.NamingRules.SourceSuffix != "" {
		sourceName += n.capitalize(n.config.NamingRules.SourceSuffix)
	}
	if n.config.NamingRules.TargetSuffix != "" {
		targetName += n.capitalize(n.config.NamingRules.TargetSuffix)
	}

	return fmt.Sprintf("Convert%sTo%s", sourceName, targetName)
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
