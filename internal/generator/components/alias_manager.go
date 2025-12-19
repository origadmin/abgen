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

var _ model.AliasManager = (*AliasManager)(nil)

// AliasManager implements the AliasManager interface.
// It is responsible for creating and managing type aliases to avoid import conflicts.
type AliasManager struct {
	config        *config.Config
	importManager model.ImportManager
	nameGenerator model.NameGenerator // The new, clean name generator
	aliasMap      map[string]string
	typeInfos     map[string]*model.TypeInfo
}

var camelCaseRegexp = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// NewAliasManager creates a new alias manager.
func NewAliasManager(
	config *config.Config,
	importManager model.ImportManager,
	nameGenerator model.NameGenerator,
	typeInfos map[string]*model.TypeInfo,
) model.AliasManager {
	return &AliasManager{
		config:        config,
		importManager: importManager,
		nameGenerator: nameGenerator,
		aliasMap:      make(map[string]string),
		typeInfos:     typeInfos,
	}
}

// PopulateAliases is the main entry point for the alias manager.
// It iterates through conversion rules and ensures all necessary types have aliases.
func (am *AliasManager) PopulateAliases() {
	slog.Debug("AliasManager: PopulateAliases started", "ruleCount", len(am.config.ConversionRules))

	for i, rule := range am.config.ConversionRules {
		slog.Debug("AliasManager: processing conversion rule", "index", i, "sourceType", rule.SourceType, "targetType", rule.TargetType)

		sourceInfo := am.typeInfos[rule.SourceType]
		targetInfo := am.typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}

		// Ensure aliases are created for the source and target types and their fields.
		am.ensureAliasesRecursively(sourceInfo, true)
		am.ensureAliasesRecursively(targetInfo, false)
	}
}

// ensureAliasesRecursively ensures that a type and all its nested types have aliases if needed.
func (am *AliasManager) ensureAliasesRecursively(typeInfo *model.TypeInfo, isSource bool) {
	if typeInfo == nil {
		return
	}

	// If an alias already exists, we assume its sub-types are also processed.
	if _, exists := am.aliasMap[typeInfo.UniqueKey()]; exists {
		return
	}

	// Generate and store the alias for the current type.
	alias := am.generateAlias(typeInfo, isSource)
	am.aliasMap[typeInfo.UniqueKey()] = alias
	slog.Debug("AliasManager: created alias", "type", typeInfo.String(), "uniqueKey", typeInfo.UniqueKey(), "alias", alias, "isSource", isSource)

	// Recursively process nested types within structs, slices, maps, etc.
	switch typeInfo.Kind {
	case model.Struct:
		for _, field := range typeInfo.Fields {
			am.ensureAliasesRecursively(field.Type, isSource)
		}
	case model.Slice, model.Array, model.Pointer:
		am.ensureAliasesRecursively(typeInfo.Underlying, isSource)
	case model.Map:
		am.ensureAliasesRecursively(typeInfo.KeyType, isSource)
		am.ensureAliasesRecursively(typeInfo.Underlying, isSource)
	}
}

// generateAlias creates a new alias for a type based on naming rules.
// This is where the core logic of naming aliases now resides.
func (am *AliasManager) generateAlias(info *model.TypeInfo, isSource bool) string {
	if info.Kind == model.Primitive {
		return am.toCamelCase(info.Name)
	}

	prefix, suffix := am.getPrefixAndSuffix(isSource)
	rawBaseName := am.getRawTypeName(info)
	processedBaseName := am.toCamelCase(rawBaseName)

	// Add type indicators for container types.
	switch info.Kind {
	case model.Slice, model.Array:
		processedBaseName += "s"
	case model.Map:
		processedBaseName += "Map"
	}

	return am.toCamelCase(prefix) + processedBaseName + am.toCamelCase(suffix)
}

func (am *AliasManager) getPrefixAndSuffix(isSource bool) (string, string) {
	if isSource {
		return am.config.NamingRules.SourcePrefix, am.config.NamingRules.SourceSuffix
	}
	return am.config.NamingRules.TargetPrefix, am.config.NamingRules.TargetSuffix
}

func (am *AliasManager) getRawTypeName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	if info.Name != "" {
		return info.Name
	}
	// Recursively find the name of the underlying type for containers.
	switch info.Kind {
	case model.Slice, model.Array, model.Pointer, model.Map:
		return am.getRawTypeName(info.Underlying)
	}
	return ""
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

// GetAliasesToRender prepares a sorted list of aliases for code generation.
func (am *AliasManager) GetAliasesToRender() []*model.AliasRenderInfo {
	var renderInfos []*model.AliasRenderInfo
	for fqn, alias := range am.aliasMap {
		typeInfo := am.typeInfos[fqn]
		if typeInfo == nil {
			continue
		}
		// Use the new NameGenerator to get the full, correct original type name.
		originalTypeName := am.nameGenerator.TypeName(typeInfo)
		renderInfos = append(renderInfos, &model.AliasRenderInfo{
			AliasName:        alias,
			OriginalTypeName: originalTypeName,
		})
	}

	// Sort for consistent output.
	sort.Slice(renderInfos, func(i, j int) bool {
		return renderInfos[i].AliasName < renderInfos[j].AliasName
	})

	return renderInfos
}

// The following methods satisfy the model.AliasManager interface but are now backed by the new logic.

func (am *AliasManager) GetSourceAlias(info *model.TypeInfo) string {
	if alias, ok := am.aliasMap[info.UniqueKey()]; ok {
		return alias
	}
	// Fallback if not populated, though it should be.
	return am.generateAlias(info, true)
}

func (am *AliasManager) GetTargetAlias(info *model.TypeInfo) string {
	if alias, ok := am.aliasMap[info.UniqueKey()]; ok {
		return alias
	}
	// Fallback if not populated.
	return am.generateAlias(info, false)
}

func (am *AliasManager) GetAllAliases() map[string]string {
	return am.aliasMap
}
