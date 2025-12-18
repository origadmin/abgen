package components

import (
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// NameGenerator implements the NameGenerator interface.
type NameGenerator struct {
	config         *config.Config
	aliasMap       map[string]string
	sourcePackages map[string]struct{}
}

var camelCaseRegexp = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// NewNameGenerator creates a new name generator.
func NewNameGenerator(config *config.Config, aliasMap map[string]string) model.NameGenerator {
	return &NameGenerator{
		config:         config,
		aliasMap:       aliasMap,
		sourcePackages: make(map[string]struct{}),
	}
}

// PopulateSourcePkgs populates the source package map from the final config.
// This should be called after all implicit rules have been discovered.
func (n *NameGenerator) PopulateSourcePkgs(config *config.Config) {
	// 1. Populate from explicit PackagePairs.
	for _, pair := range config.PackagePairs {
		n.sourcePackages[pair.SourcePath] = struct{}{}
	}

	// 2. Populate from ConversionRules' SourceType if PackagePairs is incomplete.
	for _, rule := range config.ConversionRules {
		// rule.SourceType is an FQN like "pkg/path.TypeName".
		lastDot := strings.LastIndex(rule.SourceType, ".")
		if lastDot != -1 {
			pkgPath := rule.SourceType[:lastDot]
			n.sourcePackages[pkgPath] = struct{}{}
		}
	}
}

// toCamelCase converts a string to camel case.
func (n *NameGenerator) toCamelCase(s string) string {
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
func (n *NameGenerator) GetFunctionName(source, target *model.TypeInfo) string {
	sourceName := n.getAliasedOrBaseName(source)
	targetName := n.getAliasedOrBaseName(target)
	return fmt.Sprintf("Convert%sTo%s", sourceName, targetName)
}

// GetPrimitiveConversionStubName generates a name for a primitive conversion stub function.
func (n *NameGenerator) GetPrimitiveConversionStubName(
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

// getPrefixAndSuffix returns the configured prefix and suffix for a source or target type.
func (n *NameGenerator) getPrefixAndSuffix(isSource bool) (prefix string, suffix string) {
	slog.Debug("getPrefixAndSuffix",
		"isSource", isSource,
		"SourcePrefix", n.config.NamingRules.SourcePrefix,
		"SourceSuffix", n.config.NamingRules.SourceSuffix,
		"TargetPrefix", n.config.NamingRules.TargetPrefix,
		"TargetSuffix", n.config.NamingRules.TargetSuffix,
	)
	if isSource {
		return n.config.NamingRules.SourcePrefix, n.config.NamingRules.SourceSuffix
	}
	return n.config.NamingRules.TargetPrefix, n.config.NamingRules.TargetSuffix
}

// getRawTypeName extracts the base name from a TypeInfo.
// It prioritizes the TypeInfo's own Name field.
// If the TypeInfo is an unnamed container/pointer, it recursively finds the underlying named type.
func (n *NameGenerator) getRawTypeName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	// If the type itself has a name, use it as the base name.
	// This covers structs, primitives, and named container types (e.g., `type MyList []int`).
	if info.Name != "" {
		return info.Name
	}

	// If it's an unnamed container or pointer, recursively find the underlying named type.
	switch info.Kind {
	case model.Slice, model.Array, model.Pointer, model.Map:
		if info.Underlying != nil {
			return n.getRawTypeName(info.Underlying)
		}
	}
	// Fallback for types with no name and no underlying (e.g., empty TypeInfo or primitive with empty name).
	return info.Name // This will return empty if info.Name was empty above.
}

// GetAlias generates a suitable alias for a given TypeInfo based on configured naming rules.
func (n *NameGenerator) GetAlias(info *model.TypeInfo, isSource bool) string {
	if info.Kind == model.Primitive {
		return n.toCamelCase(info.Name)
	}

	prefix, suffix := n.getPrefixAndSuffix(isSource)
	rawBaseName := n.getRawTypeName(info) // e.g., extracts "User" from "[]*User", or "UserList" from "type UserList []User"

	// 1. Convert the raw base name to camel case.
	processedBaseName := n.toCamelCase(rawBaseName) // e.g., "User" or "UserList"

	// 2. Apply smart defaults as type indicators for container types.
	// This happens before the global prefix/suffix.
	switch info.Kind {
	case model.Slice, model.Array:
		// Unconditionally append 's' as a type indicator for slices/arrays to ensure uniqueness.
		// This might result in names like "Userss" if the base name was already "Users",
		// but it guarantees distinctness from a singular struct "Users".
		processedBaseName += "s"
	case model.Map:
		processedBaseName += "Map" // e.g., "UserMap"
	}

	// 3. Apply the global prefix and suffix to the processed name.
	finalAlias := n.toCamelCase(prefix) + processedBaseName + n.toCamelCase(suffix)

	return finalAlias
}

// GetTypeString generates the full string representation of a TypeInfo,
// including pointers, slices, arrays, and maps, and handles package paths.
// This is used for the right-hand side of type alias definitions.
func (n *NameGenerator) GetTypeString(info *model.TypeInfo) string {
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
			sb.WriteString("interface{}") // Fallback if KeyType is not set.
		}
		sb.WriteString("]")
		if info.Underlying != nil { // Underlying is the value type.
			if info.Underlying.IsUltimatelyStruct() {
				sb.WriteString("*")
			}
			sb.WriteString(n.GetTypeString(info.Underlying))
		}
	default: // model.Struct, model.Primitive, etc.
		// Use BuildQualifiedTypeName to handle package paths correctly.
		info.BuildQualifiedTypeName(&sb)
	}

	return sb.String()
}

// GetTypeAliasString gets the aliased name of a type if an alias exists, otherwise returns the full type string.
// This is the primary method for generating code that should use local aliases.
func (n *NameGenerator) GetTypeAliasString(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	// First, check if an alias for the exact type exists.
	if alias, ok := n.aliasMap[info.UniqueKey()]; ok && alias != "" {
		return alias
	}

	// If no alias is found, it means it's likely a primitive, a type from the current package,
	// or another type that doesn't require an alias. In this case, we construct its full type string.
	return n.GetTypeString(info)
}

// getAliasedOrBaseName returns the alias if it exists in the map, otherwise returns the simple name.
func (n *NameGenerator) getAliasedOrBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	if alias, ok := n.aliasMap[info.UniqueKey()]; ok && alias != "" {
		slog.Debug("getAliasedOrBaseName: found cached alias", "fqn", info.FQN(), "alias", alias)
		return alias
	}

	// Recursively find the base named type to determine the package path.
	// This is crucial for composite types like slices and pointers, whose own ImportPath is empty.
	baseType := info
	for baseType != nil && (baseType.Kind == model.Slice || baseType.Kind == model.Array || baseType.Kind == model.Pointer) {
		if baseType.Underlying == nil {
			// Stop if we can't go deeper to prevent nil pointer dereference.
			break
		}
		baseType = baseType.Underlying
	}

	// Use the base type's package path to determine the logical role.
	var isSource bool
	if baseType != nil {
		_, isSource = n.sourcePackages[baseType.ImportPath]
	}

	// For debugging: print the sourcePackages keys.
	sourcePkgKeys := make([]string, 0, len(n.sourcePackages))
	for k := range n.sourcePackages {
		sourcePkgKeys = append(sourcePkgKeys, k)
	}
	sort.Strings(sourcePkgKeys) // Sort for consistent log order.

	slog.Debug("getAliasedOrBaseName: determining isSource",
		"original_fqn", info.FQN(),
		"baseType_fqn", baseType.FQN(),
		"baseType_importPath", baseType.ImportPath,
		"sourcePackages_keys", sourcePkgKeys,
		"determined_isSource", isSource,
	)

	// Call GetAlias with the original type 'info', but with the correctly determined 'isSource' flag.
	return n.GetAlias(info, isSource)
}
