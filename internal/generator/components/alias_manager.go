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

// nonManagedPackages contains a set of standard library or common third-party packages
// that should never have their types aliased.
var nonManagedPackages = map[string]struct{}{
	"time":           {},
	"encoding/json":  {},
	"github.com/google/uuid": {},
}

// AliasManager implements the AliasManager interface.
type AliasManager struct {
	config              *config.Config
	importManager       model.ImportManager
	aliasMap            map[string]string
	typeInfos           map[string]*model.TypeInfo
	aliasedTypes        map[string]*model.TypeInfo
	managedPackagePaths map[string]struct{}
	fqnToExistingAlias  map[string]string
	visited             map[string]bool
}

var camelCaseRegexp = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// NewAliasManager creates a new alias manager.
func NewAliasManager(
	config *config.Config,
	importManager model.ImportManager,
	typeInfos map[string]*model.TypeInfo,
) model.AliasManager {
	// Create the reverse map from FQN to existing alias name
	fqnToAlias := make(map[string]string, len(config.ExistingAliases))
	for alias, fqn := range config.ExistingAliases {
		fqnToAlias[fqn] = alias
	}

	return &AliasManager{
		config:              config,
		importManager:       importManager,
		aliasMap:            make(map[string]string),
		typeInfos:           typeInfos,
		aliasedTypes:        make(map[string]*model.TypeInfo),
		managedPackagePaths: make(map[string]struct{}),
		fqnToExistingAlias:  fqnToAlias,
		visited:             make(map[string]bool),
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
	// Infer "managed" packages from conversion rules, excluding blacklisted packages.
	for _, rule := range am.config.ConversionRules {
		am.addManagedPackage(getPkgPath(rule.SourceType))
		am.addManagedPackage(getPkgPath(rule.TargetType))
	}
	slog.Debug("AliasManager: collected managed packages for aliasing", "paths", am.managedPackagePaths)

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

// addManagedPackage adds a package path to the set of managed packages, if it's not empty and not blacklisted.
func (am *AliasManager) addManagedPackage(pkgPath string) {
	if pkgPath == "" {
		return
	}
	if _, isExcluded := nonManagedPackages[pkgPath]; isExcluded {
		return
	}
	am.managedPackagePaths[pkgPath] = struct{}{}
}

// ensureAliasesRecursively ensures that a type and all its nested types have aliases if needed.
func (am *AliasManager) ensureAliasesRecursively(typeInfo *model.TypeInfo, isSource bool) {
	if typeInfo == nil {
		return
	}

	uniqueKey := typeInfo.UniqueKey()
	if am.visited[uniqueKey] {
		return
	}
	am.visited[uniqueKey] = true

	// Recurse on component types first.
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
	case model.Pointer:
		am.ensureAliasesRecursively(typeInfo.Underlying, isSource)
	}

	// After recursing, decide if THIS type needs an alias.
	if !am.isManagedType(typeInfo) {
		return
	}

	// CRITICAL: Do not generate aliases for pointer types themselves.
	if typeInfo.Kind == model.Pointer {
		return
	}

	// Check if an alias already exists from user directives.
	if existingAlias, ok := am.fqnToExistingAlias[uniqueKey]; ok {
		am.aliasMap[uniqueKey] = existingAlias
		slog.Debug("AliasManager: using existing user-defined alias", "type", typeInfo.String(), "uniqueKey", uniqueKey, "alias", existingAlias)
		return
	}

	// Generate and store the alias.
	alias := am.generateAlias(typeInfo, isSource)
	am.aliasMap[uniqueKey] = alias
	am.aliasedTypes[uniqueKey] = typeInfo
	slog.Debug("AliasManager: created alias", "type", typeInfo.String(), "uniqueKey", uniqueKey, "alias", alias)
}

// isManagedType recursively checks if a type or any of its component types
// belong to the source or target packages defined in the configuration.
func (am *AliasManager) isManagedType(info *model.TypeInfo) bool {
	if info == nil {
		return false
	}

	switch info.Kind {
	case model.Primitive:
		return false
	case model.Pointer:
		return am.isManagedType(info.Underlying)
	case model.Named, model.Struct:
		_, isManaged := am.managedPackagePaths[info.ImportPath]
		return isManaged
	case model.Slice, model.Array:
		return am.isManagedType(info.Underlying)
	case model.Map:
		return am.isManagedType(info.KeyType) || am.isManagedType(info.Underlying)
	default:
		return false
	}
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
		return am.getCleanBaseNameForAlias(info.Underlying)
	case model.Slice:
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
	baseName := am.getCleanBaseNameForAlias(info)
	prefix, suffix := am.getPrefixAndSuffix(isSource)
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

// getPkgPath extracts the package path from a fully-qualified type name.
func getPkgPath(fqn string) string {
	lastDot := strings.LastIndex(fqn, ".")
	if lastDot == -1 {
		return ""
	}
	return fqn[:lastDot]
}
