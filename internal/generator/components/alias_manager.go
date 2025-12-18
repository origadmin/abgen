package components

import (
	"log/slog"
	"sort"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

var _ model.AliasManager = (*AliasManager)(nil)

// AliasManager implements the AliasManager interface.
type AliasManager struct {
	config           *config.Config
	nameGenerator    model.NameGenerator
	importManager    model.ImportManager  // 新增：导入管理器
	aliasMap         map[string]string
	requiredAliases  map[string]struct{}
	typeInfos        map[string]*model.TypeInfo
}

// NewAliasManager creates a new alias manager.
func NewAliasManager(
	config *config.Config,
	nameGenerator model.NameGenerator,
	importManager model.ImportManager,  // 新增：导入管理器参数
	typeInfos map[string]*model.TypeInfo,
) model.AliasManager {
	return &AliasManager{
		config:          config,
		nameGenerator:   nameGenerator,
		importManager:   importManager,  // 新增：初始化导入管理器
		aliasMap:        make(map[string]string),
		requiredAliases: make(map[string]struct{}),
		typeInfos:       typeInfos,
	}
}

// EnsureTypeAlias ensures that the specified type has an alias, creating one if it doesn't exist.
func (am *AliasManager) EnsureTypeAlias(typeInfo *model.TypeInfo, isSource bool) {
	if typeInfo == nil {
		return
	}

	uniqueKey := typeInfo.UniqueKey()

	// If the alias already exists, return immediately.
	if _, exists := am.aliasMap[uniqueKey]; exists {
		return
	}

	// Create a new alias.
	alias := am.nameGenerator.GetAlias(typeInfo, isSource)

	// Store it in the map immediately.
	am.aliasMap[uniqueKey] = alias
	am.requiredAliases[uniqueKey] = struct{}{}

	slog.Debug("AliasManager: ensured type alias",
		"type", typeInfo.String(),
		"uniqueKey", uniqueKey,
		"alias", alias,
		"isSource", isSource)

	// Recursively handle element types of composite types.
	am.ensureElementTypeAlias(typeInfo, isSource)
}

// ensureElementTypeAlias recursively ensures aliases for element types.
func (am *AliasManager) ensureElementTypeAlias(typeInfo *model.TypeInfo, isSource bool) {
	switch typeInfo.Kind {
	case model.Slice, model.Array, model.Pointer:
		if typeInfo.Underlying != nil {
			am.EnsureTypeAlias(typeInfo.Underlying, isSource)
		}
	case model.Map:
		if typeInfo.KeyType != nil {
			am.EnsureTypeAlias(typeInfo.KeyType, isSource)
		}
		if typeInfo.Underlying != nil {
			am.EnsureTypeAlias(typeInfo.Underlying, isSource)
		}
	case model.Struct:
		// Recursively create aliases for named struct field types.
		for _, field := range typeInfo.Fields {
			am.EnsureTypeAlias(field.Type, isSource)
		}
	}
}

// GetAliasMap returns the alias map.
func (am *AliasManager) GetAliasMap() map[string]string {
	return am.aliasMap
}

// GetRequiredAliases returns the set of required aliases.
func (am *AliasManager) GetRequiredAliases() map[string]struct{} {
	return am.requiredAliases
}

// PopulateAliases populates aliases for all conversion rules.
func (am *AliasManager) PopulateAliases() {
	for _, rule := range am.config.ConversionRules {
		sourceInfo := am.typeInfos[rule.SourceType]
		targetInfo := am.typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}

		// Ensure base types have aliases.
		am.EnsureTypeAlias(sourceInfo, true)
		am.EnsureTypeAlias(targetInfo, false)
		
		// 新增：添加源包和目标包的导入
		if sourceInfo.ImportPath != "" && !am.isCurrentPackage(sourceInfo.ImportPath) {
			am.importManager.Add(sourceInfo.ImportPath)
		}
		if targetInfo.ImportPath != "" && !am.isCurrentPackage(targetInfo.ImportPath) {
			am.importManager.Add(targetInfo.ImportPath)
		}
	}
}

// isCurrentPackage checks if the import path belongs to the current package
func (am *AliasManager) isCurrentPackage(importPath string) bool {
	return importPath == am.config.GenerationContext.PackagePath
}

// GetAliasesToRender prepares and returns a sorted list of aliases that need to be rendered in the generated code.
func (am *AliasManager) GetAliasesToRender() []*model.AliasRenderInfo {
	type aliasPair struct {
		aliasName, fqn string
	}
	aliasesToWrite := make([]aliasPair, 0)

	for fqn, alias := range am.aliasMap {
		// Only consider aliases that are actually required.
		if _, ok := am.requiredAliases[fqn]; !ok {
			continue
		}
		// Skip aliases that are already defined in the config.
		if _, exists := am.config.ExistingAliases[alias]; exists {
			continue
		}
		// Skip aliases for types that were not resolved.
		if am.typeInfos[fqn] == nil {
			continue
		}
		aliasesToWrite = append(aliasesToWrite, aliasPair{alias, fqn})
	}

	// Sort for consistent output.
	sort.Slice(aliasesToWrite, func(i, j int) bool {
		return aliasesToWrite[i].aliasName < aliasesToWrite[j].aliasName
	})

	// Convert to the render info struct.
	renderInfos := make([]*model.AliasRenderInfo, 0, len(aliasesToWrite))
	for _, item := range aliasesToWrite {
		typeInfo := am.typeInfos[item.fqn]
		originalTypeName := am.nameGenerator.GetTypeString(typeInfo)
		renderInfos = append(renderInfos, &model.AliasRenderInfo{
			AliasName:        item.aliasName,
			OriginalTypeName: originalTypeName,
		})
	}

	return renderInfos
}