package components

import (
	"go/types"
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
	}
}

// GetAlias looks up an alias for a given *types.Named type.
// This is used by the TypeFormatter to check if a named type should be replaced with an alias.
func (am *AliasManager) GetAlias(t *types.Named) (string, bool) {
	pkg := t.Obj().Pkg()
	if pkg == nil {
		// Built-in types (like 'error') or unnamed types don't have aliases in this system.
		return "", false
	}
	// Reconstruct the unique key from the types.Type object.
	uniqueKey := pkg.Path() + "." + t.Obj().Name()
	return am.LookupAlias(uniqueKey)
}

// LookupAlias implements the model.AliasLookup interface.
func (am *AliasManager) LookupAlias(uniqueKey string) (string, bool) {
	alias, ok := am.aliasMap[uniqueKey]
	return alias, ok
}

// PopulateAliases is the main entry point for the alias manager.
func (am *AliasManager) PopulateAliases() {
	slog.Debug("AliasManager: PopulateAliases started", "ruleCount", len(am.config.ConversionRules))

	for i, rule := range am.config.ConversionRules {
		slog.Debug("AliasManager: processing conversion rule", "index", i, "sourceType", rule.SourceType, "targetType", rule.TargetType)

		sourceInfo := am.typeInfos[rule.SourceType]
		targetInfo := am.typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}

		am.ensureAliasesRecursively(sourceInfo, true)
		am.ensureAliasesRecursively(targetInfo, false)
	}
}

// ensureAliasesRecursively ensures that a type and all its nested types have aliases if needed.
func (am *AliasManager) ensureAliasesRecursively(typeInfo *model.TypeInfo, isSource bool) {
	if typeInfo == nil {
		return
	}

	if _, exists := am.aliasMap[typeInfo.UniqueKey()]; exists {
		return
	}

	alias := am.generateAlias(typeInfo, isSource)
	am.aliasMap[typeInfo.UniqueKey()] = alias
	slog.Debug("AliasManager: created alias", "type", typeInfo.String(), "uniqueKey", typeInfo.UniqueKey(), "alias", alias, "isSource", isSource)

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
// GenerateAliasForTesting is a helper method for testing that generates an alias for the given type.
// This is primarily used in tests to verify alias generation logic.
func (am *AliasManager) GenerateAliasForTesting(info *model.TypeInfo, isSource bool) string {
	return am.generateAlias(info, isSource)
}

func (am *AliasManager) generateAlias(info *model.TypeInfo, isSource bool) string {
	if info.Kind == model.Primitive {
		return am.toCamelCase(info.Name)
	}

	prefix, suffix := am.getPrefixAndSuffix(isSource)
	rawBaseName := am.getRawTypeName(info)
	processedBaseName := am.toCamelCase(rawBaseName)

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

func (am *AliasManager) GetAllAliases() map[string]string {
	return am.aliasMap
}
