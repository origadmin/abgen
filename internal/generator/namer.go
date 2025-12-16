package generator

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// Namer is responsible for generating names for functions and type aliases.
type Namer struct {
	config   *config.Config
	aliasMap map[string]string
}

var camelCaseRegexp = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// NewNamer creates a new Namer.
func NewNamer(config *config.Config, aliasMap map[string]string) *Namer {
	return &Namer{
		config:   config,
		aliasMap: aliasMap,
	}
}

// toCamelCase converts a string to CamelCase.
func (n *Namer) toCamelCase(s string) string {
	if s == "" {
		return ""
	}
	s = camelCaseRegexp.ReplaceAllString(s, " ")
	parts := strings.Fields(s)
	for i := range parts {
		if len(parts[i]) > 0 {
			runes := []rune(parts[i])
			runes[0] = unicode.ToUpper(runes[0])
			parts[i] = string(runes)
		}
	}
	return strings.Join(parts, "")
}

// GetFunctionName generates a name for a conversion function.
func (n *Namer) GetFunctionName(source, target *model.TypeInfo) string {
	sourceName := n.getAliasedOrBaseName(source)
	targetName := n.getAliasedOrBaseName(target)
	return fmt.Sprintf("Convert%sTo%s", sourceName, targetName)
}

// GetPrimitiveConversionStubName generates a name for a primitive conversion stub function.
func (n *Namer) GetPrimitiveConversionStubName(
	parentSource *model.TypeInfo, sourceField *model.FieldInfo,
	parentTarget *model.TypeInfo, targetField *model.FieldInfo,
) string {
	parentSourceName := n.getAliasedOrBaseName(parentSource)
	parentTargetName := n.getAliasedOrBaseName(parentTarget)
	sourceFieldName := n.toCamelCase(sourceField.Name)
	targetFieldName := n.toCamelCase(targetField.Name)
	return fmt.Sprintf("Convert%s%sTo%s%s",
		parentSourceName, sourceFieldName,
		parentTargetName, targetFieldName,
	)
}

// getPrefixAndSuffix returns the configured prefix and suffix for either a source or a target type.
func (n *Namer) getPrefixAndSuffix(isSource bool) (prefix string, suffix string) {
	if isSource {
		return n.config.NamingRules.SourcePrefix, n.config.NamingRules.SourceSuffix
	}
	return n.config.NamingRules.TargetPrefix, n.config.NamingRules.TargetSuffix
}

// getRawTypeName extracts the base name from TypeInfo.
// It prioritizes the TypeInfo's own Name field if present.
// If the TypeInfo is an unnamed container/pointer, it recurses to find the underlying named type.
func (n *Namer) getRawTypeName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	// If the type itself has a name, use it as the base name.
	// This covers structs, primitives, and *named* container types (e.g., `type MyList []int`).
	if info.Name != "" {
		return info.Name
	}

	// If it's an *unnamed* container or pointer, recurse to find the underlying named type.
	switch info.Kind {
	case model.Slice, model.Array, model.Pointer, model.Map:
		if info.Underlying != nil {
			return n.getRawTypeName(info.Underlying)
		}
	}
	// Fallback for types without a name and no underlying (e.g., an empty TypeInfo, or a primitive with empty name)
	return info.Name // This will be empty if info.Name was empty above and no underlying was found.
}

// GetAlias generates a suitable alias for a given TypeInfo based on configured naming rules.
func (n *Namer) GetAlias(info *model.TypeInfo, isSource bool) string {
	if info.Kind == model.Primitive {
		return n.toCamelCase(info.Name)
	}

	prefix, suffix := n.getPrefixAndSuffix(isSource)
	rawBaseName := n.getRawTypeName(info) // e.g., "User" from "[]*User", or "UserList" from "type UserList []User"

	// 1. Convert raw base name to CamelCase.
	processedBaseName := n.toCamelCase(rawBaseName) // e.g., "User" or "UserList"

	// 2. Apply intelligent defaults for container types as type indicators.
	// This happens *before* global prefix/suffix.
	switch info.Kind {
	case model.Slice, model.Array:
		// Unconditionally append 's' as a type indicator for slices/arrays to ensure uniqueness.
		// This might result in names like "Userss" if the base name is already "Users",
		// but it guarantees distinctness from the singular struct "Users".
		processedBaseName += "s"
	case model.Map:
		processedBaseName += "Map" // e.g., "UserMap"
	}

	// 3. Apply the global prefix and suffix to the processed name.
	finalAlias := n.toCamelCase(prefix) + processedBaseName + n.toCamelCase(suffix)

	return finalAlias
}

// GetTypeString generates the full string representation of a TypeInfo,
// including pointers, slices, arrays, and maps, and handling package paths.
// This is used for the right-hand side of type alias definitions.
func (n *Namer) GetTypeString(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	var sb strings.Builder

	switch info.Kind {
	case model.Pointer:
		sb.WriteString("*")
		if info.Underlying != nil {
			sb.WriteString(n.GetTypeString(info.Underlying))
		}
	case model.Slice:
		sb.WriteString("[]")
		if info.Underlying != nil {
			// If the underlying type of a slice is ultimately a struct, it should be a pointer in the slice.
			if info.Underlying.IsUltimatelyStruct() {
				sb.WriteString("*")
			}
			sb.WriteString(n.GetTypeString(info.Underlying))
		}
	case model.Array:
		sb.WriteString(fmt.Sprintf("[%d]", info.ArrayLen)) // Use ArrayLen
		if info.Underlying != nil {
			if info.Underlying.IsUltimatelyStruct() {
				sb.WriteString("*")
			}
			sb.WriteString(n.GetTypeString(info.Underlying))
		}
	case model.Map:
		sb.WriteString("map[")
		if info.KeyType != nil { // Use KeyType
			sb.WriteString(n.GetTypeString(info.KeyType))
		} else {
			sb.WriteString("interface{}") // Fallback if KeyType is not set
		}
		sb.WriteString("]")
		if info.Underlying != nil { // Underlying is the value type
			if info.Underlying.IsUltimatelyStruct() {
				sb.WriteString("*")
			}
			sb.WriteString(n.GetTypeString(info.Underlying))
		}
	default: // model.Struct, model.Primitive, etc.
		// Use BuildQualifiedTypeName to correctly handle package paths
		info.BuildQualifiedTypeName(&sb)
	}

	return sb.String()
}

// GetTypeAliasString gets the aliased name for a type if one exists, otherwise returns the full type string.
// This is the primary method to use when generating code that should use local aliases.
func (n *Namer) GetTypeAliasString(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	// First, check if an alias exists for the exact type.
	if alias, ok := n.aliasMap[info.UniqueKey()]; ok && alias != "" {
		return alias
	}

	// If no alias is found, it means it's either a primitive, a type from the current package,
	// or something that doesn't need an alias. In this case, we construct its full type string.
	return n.GetTypeString(info)
}

// getAliasedOrBaseName returns the alias if it exists in the map, or the simple name.
func (n *Namer) getAliasedOrBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	if alias, ok := n.aliasMap[info.UniqueKey()]; ok {
		return alias
	}
	// For function naming, we generate the alias on the fly.
	// We assume 'source' as a default context. This is a simplification, but
	// sufficient for creating consistent function names.
	return n.GetAlias(info, true)
}
