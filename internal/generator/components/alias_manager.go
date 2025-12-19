package components

import (
	"log/slog"
	"regexp"
	"strings"
	"unicode"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

var _ model.AliasManager = (*AliasManager)(nil)

// AliasManager implements the AliasManager interface.
type AliasManager struct {
	config        *config.Config
	importManager model.ImportManager
	aliasMap      map[string]string
	typeInfos     map[string]*model.TypeInfo
	aliasedTypes  map[string]*model.TypeInfo
}

var camelCaseRegexp = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// NewAliasManager creates a new alias manager.
func NewAliasManager(
	config *config.Config,
	importManager model.ImportManager,
	typeInfos map[string]*model.TypeInfo,
) model.AliasManager {
	return &AliasManager{
		config:        config,
		importManager: importManager,
		aliasMap:      make(map[string]string),
		typeInfos:     typeInfos,
		aliasedTypes:  make(map[string]*model.TypeInfo),
	}
}

// GetAliasedTypes returns the map of aliased types.
func (am *AliasManager) GetAliasedTypes() map[string]*model.TypeInfo {
	return am.aliasedTypes
}

// GetAlias looks up an alias for a given *model.TypeInfo.
func (am *AliasManager) GetAlias(info *model.TypeInfo) (string, bool) {
	if info == nil {
		return "", false
	}
	if info.Kind == model.Primitive || info.Kind == model.Pointer {
		return "", false
	}
	return am.LookupAlias(info.UniqueKey())
}

// LookupAlias implements the model.AliasLookup interface.
func (am *AliasManager) LookupAlias(uniqueKey string) (string, bool) {
	alias, ok := am.aliasMap[uniqueKey]
	return alias, ok
}

// PopulateAliases is the main entry point for the alias manager.
func (am *AliasManager) PopulateAliases() {
	for _, rule := range am.config.ConversionRules {
		sourceInfo := am.typeInfos[rule.SourceType]
		targetInfo := am.typeInfos[rule.TargetType]
		if sourceInfo != nil {
			am.ensureAliasesRecursively(sourceInfo, true)
		}
		if targetInfo != nil {
			am.ensureAliasesRecursively(targetInfo, false)
		}
	}
}

// ensureAliasesRecursively ensures that a type and all its nested types have aliases if needed.
func (am *AliasManager) ensureAliasesRecursively(typeInfo *model.TypeInfo, isSource bool) {
	if typeInfo == nil {
		return
	}

	// CRITICAL: Check if alias already exists to break recursion cycles.
	if _, exists := am.aliasMap[typeInfo.UniqueKey()]; exists {
		return
	}

	// For types that we won't generate an alias for (primitives, pointers),
	// we just need to recurse on their children and then return.
	if typeInfo.Kind == model.Primitive {
		return // Primitives don't have children and don't get aliases.
	}
	if typeInfo.Kind == model.Pointer {
		am.ensureAliasesRecursively(typeInfo.Underlying, isSource)
		return // Pointers themselves don't get aliases.
	}

	// For all other types (Struct, Slice, Map, etc.), we generate an alias.
	// CRITICAL: Add a placeholder to the map *before* recursing to handle cyclic dependencies.
	am.aliasMap[typeInfo.UniqueKey()] = "placeholder"

	// Recurse on component types.
	switch typeInfo.Kind {
	case model.Struct:
		for _, field := range typeInfo.Fields {
			am.ensureAliasesRecursively(field.Type, isSource)
		}
	case model.Slice, model.Array:
		am.ensureAliasesRecursively(typeInfo.Underlying, isSource)
	case model.Map:
		am.ensureAliasesRecursively(typeInfo.KeyType, isSource)
		am.ensureAliasesRecursively(typeInfo.Underlying, isSource)
	}

	// Now that all children are processed, generate the real alias.
	alias := am.generateAlias(typeInfo, isSource)

	// Update the map with the real alias.
	am.aliasMap[typeInfo.UniqueKey()] = alias
	am.aliasedTypes[typeInfo.UniqueKey()] = typeInfo
	slog.Debug("AliasManager: created alias", "type", typeInfo.String(), "uniqueKey", typeInfo.UniqueKey(), "alias", alias)
}


// getCleanBaseNameForAlias recursively builds a clean, suffix-free base name for a type.
func (am *AliasManager) getCleanBaseNameForAlias(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	switch info.Kind {
	case model.Primitive:
		return am.toCamelCase(info.Name)
	case model.Named, model.Struct:
		return am.toCamelCase(info.Name)
	case model.Pointer:
		// Pointers are transparent for naming purposes.
		return am.getCleanBaseNameForAlias(info.Underlying)
	case model.Slice:
		// Pluralize the underlying type's base name.
		return am.getCleanBaseNameForAlias(info.Underlying) + "s"
	case model.Array:
		return am.getCleanBaseNameForAlias(info.Underlying) + "Array"
	case model.Map:
		keyBaseName := am.getCleanBaseNameForAlias(info.KeyType)
		valueBaseName := am.getCleanBaseNameForAlias(info.Underlying)
		return keyBaseName + "To" + valueBaseName + "Map"
	default:
		return "Unknown"
	}
}

// generateAlias creates a new alias for a type based on naming rules.
func (am *AliasManager) generateAlias(info *model.TypeInfo, isSource bool) string {
	// 1. Get the clean, unprocessed base name.
	baseName := am.getCleanBaseNameForAlias(info)

	// 2. Get the appropriate prefix and suffix.
	prefix, suffix := am.getPrefixAndSuffix(isSource)

	// 3. Combine them into the final alias.
	finalAlias := am.toCamelCase(prefix) + baseName + am.toCamelCase(suffix)
	slog.Debug("AliasManager: final alias generated", "type", info.String(), "baseName", baseName, "finalAlias", finalAlias)
	return finalAlias
}

func (am *AliasManager) getPrefixAndSuffix(isSource bool) (string, string) {
	if isSource {
		return am.config.NamingRules.SourcePrefix, am.config.NamingRules.SourceSuffix
	}
	return am.config.NamingRules.TargetPrefix, am.config.NamingRules.TargetSuffix
}

func (am *AliasManager) toCamelCase(s string) string {
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

func (am *AliasManager) GetAllAliases() map[string]string {
	return am.aliasMap
}

func (am *AliasManager) GetSourcePath() string {
	if len(am.config.PackagePairs) > 0 {
		return am.config.PackagePairs[0].SourcePath
	}
	return ""
}

func (am *AliasManager) GetTargetPath() string {
	if len(am.config.PackagePairs) > 0 {
		return am.config.PackagePairs[0].TargetPath
	}
	return ""
}
