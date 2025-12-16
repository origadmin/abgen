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
// It handles cases like "user_custom" -> "UserCustom", "user-custom" -> "UserCustom".
// For "usercustom", it will return "Usercustom". It does not attempt to split words
// unless there's a non-alphanumeric separator.
func (n *Namer) toCamelCase(s string) string {
	if s == "" {
		return ""
	}

	// Replace non-alphanumeric characters with spaces
	s = camelCaseRegexp.ReplaceAllString(s, " ")
	// Capitalize first letter of each word and join
	parts := strings.Fields(s) // Split by spaces
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

// GetPrimitiveConversionStubName generates a name for a primitive conversion stub function,
// incorporating parent type and field names for better context.
// The naming rule is: Convert + ParentSourceTypeAlias + SourceFieldName + To + ParentTargetTypeAlias + TargetFieldName
func (n *Namer) GetPrimitiveConversionStubName(
	parentSource *model.TypeInfo, sourceField *model.FieldInfo,
	parentTarget *model.TypeInfo, targetField *model.FieldInfo,
) string {
	parentSourceName := n.getAliasedOrBaseName(parentSource)
	parentTargetName := n.getAliasedOrBaseName(parentTarget)

	// Field names are directly camel-cased, they don't have configured prefixes/suffixes.
	sourceFieldName := n.toCamelCase(sourceField.Name)
	targetFieldName := n.toCamelCase(targetField.Name)

	return fmt.Sprintf("Convert%s%sTo%s%s",
		parentSourceName, sourceFieldName,
		parentTargetName, targetFieldName,
	)
}

// getEffectivePrefixAndSuffix returns the configured prefix and suffix for a given type direction.
func (n *Namer) getEffectivePrefixAndSuffix(isSource bool) (string, string) {
	if isSource {
		return n.config.NamingRules.SourcePrefix, n.config.NamingRules.SourceSuffix
	}
	return n.config.NamingRules.TargetPrefix, n.config.NamingRules.TargetSuffix
}

// getRawTypeName extracts the raw base name from TypeInfo.
// It recursively gets the name of the underlying named type for composite types.
func (n *Namer) getRawTypeName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	// If it's a named type, return its name directly.
	if info.IsNamedType() {
		return info.Name
	}

	// For composite types, recursively get the name of the underlying type.
	switch info.Kind {
	case model.Slice, model.Array, model.Pointer:
		if info.Underlying != nil {
			return n.getRawTypeName(info.Underlying)
		}
	case model.Map:
		// For maps, we might need to consider both key and value types,
		// but for a single "raw base name", we typically refer to the value type.
		if info.Underlying != nil {
			return n.getRawTypeName(info.Underlying)
		}
	}

	// Fallback for primitive types or unhandled cases.
	return info.Name
}

// GetAlias generates a suitable alias for a given TypeInfo, applying prefixes, suffixes, and disambiguation.
func (n *Namer) GetAlias(info *model.TypeInfo, isSource, needsDisambiguation bool) string {
	if info.Kind == model.Primitive {
		return n.toCamelCase(info.Name)
	}

	// 1. 获取最底层命名类型名称 (e.g., "User", "Item", "Resource")
	rawBaseName := n.getRawTypeName(info)

	// 2. 获取当前方向 (source/target) 的配置前缀和后缀
	configuredPrefix, configuredSuffix := n.getEffectivePrefixAndSuffix(isSource)

	// 3. 驼峰化 rawBaseName
	camelCasedBase := n.toCamelCase(rawBaseName)

	// 4. 拼接别名 (应用配置的前缀/后缀)
	// 配置的前缀/后缀应该应用于最底层的命名类型，而不是复合类型本身。
	// 但是，GetAlias 的 info 参数可能是复合类型，所以这里直接应用到 camelCasedBase。
	// 最终的别名会是 CamelCasedPrefix + CamelCasedBase + CamelCasedSuffix
	finalAlias := n.toCamelCase(configuredPrefix) + camelCasedBase + n.toCamelCase(configuredSuffix)

	// 5. 处理切片复数化
	if info.Kind == model.Slice {
		finalAlias += "s"
	}

	// 6. 应用歧义后缀
	// 歧义后缀仅在 needsDisambiguation 为 true 时应用。
	// needsDisambiguation 标志本身已经包含了“没有配置任何前缀/后缀”的判断。
	if needsDisambiguation {
		if isSource {
			finalAlias += "Source"
		} else {
			finalAlias += "Target"
		}
	}

	return finalAlias
}

// getAliasedOrBaseName returns the alias if it exists in the map, otherwise generates and returns it.
// This is the primary function to get a type's name for use in generated code.
func (n *Namer) getAliasedOrBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	key := info.UniqueKey()
	if alias, ok := n.aliasMap[key]; ok {
		return alias
	}

	// Fallback: if not found in aliasMap, return the simple camel-cased name.
	// This path is typically taken for types that are not explicitly part of a conversion rule
	// or its direct nested types that require a special alias.
	// They should just use their canonical name or a simple camel-cased version.
	return n.toCamelCase(info.Name)
}
