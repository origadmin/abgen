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
	config            *config.Config
	nameGenerator     model.NameGenerator
	importManager     model.ImportManager // 新增：导入管理器
	aliasMap          map[string]string
	requiredAliases   map[string]struct{}
	typeInfos         map[string]*model.TypeInfo
	fieldTypesToAlias map[string]*model.TypeInfo
}

// NewAliasManager creates a new alias manager.
func NewAliasManager(
	config *config.Config,
	nameGenerator model.NameGenerator,
	importManager model.ImportManager, // 新增：导入管理器参数
	typeInfos map[string]*model.TypeInfo,
) model.AliasManager {
	return &AliasManager{
		config:          config,
		nameGenerator:   nameGenerator,
		importManager:   importManager, // 新增：初始化导入管理器
		aliasMap:        make(map[string]string),
		requiredAliases: make(map[string]struct{}),
		typeInfos:       typeInfos,
	}
}

// EnsureTypeAlias ensures that the specified type has an alias, creating one if it doesn't exist.
func (am *AliasManager) EnsureTypeAlias(typeInfo *model.TypeInfo, isSource bool) {
	if typeInfo == nil {
		slog.Debug("AliasManager: EnsureTypeAlias called with nil typeInfo")
		return
	}

	uniqueKey := typeInfo.UniqueKey()

	// If the alias already exists, return immediately.
	if _, exists := am.aliasMap[uniqueKey]; exists {
		slog.Debug("AliasManager: alias already exists",
			"uniqueKey", uniqueKey,
			"alias", am.aliasMap[uniqueKey])
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
		"isSource", isSource,
		"kind", typeInfo.Kind.String())

	// Recursively handle element types of composite types.
	am.ensureElementTypeAlias(typeInfo, isSource)
}

// ensureElementTypeAlias recursively ensures aliases for element types.
func (am *AliasManager) ensureElementTypeAlias(typeInfo *model.TypeInfo, isSource bool) {
	slog.Debug("AliasManager: ensureElementTypeAlias",
		"type", typeInfo.String(),
		"kind", typeInfo.Kind.String())

	switch typeInfo.Kind {
	case model.Slice, model.Array, model.Pointer:
		if typeInfo.Underlying != nil {
			slog.Debug("AliasManager: processing container element",
				"containerType", typeInfo.String(),
				"elementType", typeInfo.Underlying.String())
			am.EnsureTypeAlias(typeInfo.Underlying, isSource)
			// 递归处理嵌套的容器类型
			am.ensureElementTypeAlias(typeInfo.Underlying, isSource)
		} else {
			slog.Debug("AliasManager: container type has nil underlying type",
				"type", typeInfo.String())
		}
	case model.Map:
		if typeInfo.KeyType != nil {
			am.EnsureTypeAlias(typeInfo.KeyType, isSource)
		}
		if typeInfo.Underlying != nil {
			am.EnsureTypeAlias(typeInfo.Underlying, isSource)
		}
	case model.Struct:
		// 递归处理结构体字段
		slog.Debug("AliasManager: processing struct fields",
			"structType", typeInfo.String(),
			"fieldCount", len(typeInfo.Fields))
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
	slog.Debug("AliasManager: PopulateAliases started",
		"ruleCount", len(am.config.ConversionRules))

	for i, rule := range am.config.ConversionRules {
		slog.Debug("AliasManager: processing conversion rule",
			"index", i,
			"sourceType", rule.SourceType,
			"targetType", rule.TargetType)

		sourceInfo := am.typeInfos[rule.SourceType]
		targetInfo := am.typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			slog.Debug("AliasManager: skipping rule - type info not found",
				"sourceInfoNil", sourceInfo == nil,
				"targetInfoNil", targetInfo == nil)
			continue
		}

		slog.Debug("AliasManager: ensuring base type aliases",
			"sourceType", sourceInfo.String(),
			"targetType", targetInfo.String())

		// Ensure base types have aliases.
		am.EnsureTypeAlias(sourceInfo, true)
		am.EnsureTypeAlias(targetInfo, false)

		// 新增：递归处理结构体字段中的类型别名
		am.ensureFieldTypeAliases(sourceInfo, true)
		am.ensureFieldTypeAliases(targetInfo, false)

		// 新增：添加源包和目标包的导入
		if sourceInfo.ImportPath != "" && !am.isCurrentPackage(sourceInfo.ImportPath) {
			am.importManager.Add(sourceInfo.ImportPath)
		}
		if targetInfo.ImportPath != "" && !am.isCurrentPackage(targetInfo.ImportPath) {
			am.importManager.Add(targetInfo.ImportPath)
		}
	}
}

// ensureFieldTypeAliases recursively ensures aliases for all field types in a struct
func (am *AliasManager) ensureFieldTypeAliases(typeInfo *model.TypeInfo, isSource bool) {
	if typeInfo == nil {
		slog.Debug("AliasManager: ensureFieldTypeAliases called with nil typeInfo")
		return
	}

	slog.Debug("AliasManager: ensureFieldTypeAliases",
		"type", typeInfo.String(),
		"kind", typeInfo.Kind.String())

	// 对于结构体类型，递归处理所有字段
	if typeInfo.Kind == model.Struct {
		slog.Debug("AliasManager: processing struct fields",
			"structType", typeInfo.String(),
			"fieldCount", len(typeInfo.Fields))
		for i, field := range typeInfo.Fields {
			if field.Type != nil {
				slog.Debug("AliasManager: ensuring field type alias",
					"fieldIndex", i,
					"fieldName", field.Name,
					"fieldType", field.Type.String(),
					"fieldTypeKind", field.Type.Kind.String())
				// 确保字段类型本身有别名
				am.EnsureTypeAlias(field.Type, isSource)
				// 递归处理字段类型的所有嵌套结构（包括指针、切片等）
				am.ensureElementTypeAlias(field.Type, isSource)
			}
		}
	}
}

// isCurrentPackage checks if the import path belongs to the current package
func (am *AliasManager) isCurrentPackage(importPath string) bool {
	return importPath == am.config.GenerationContext.PackagePath
}

// GetAliasesToRender prepares and returns a sorted list of aliases that need to be rendered in the generated code.
func (am *AliasManager) GetAliasesToRender() []*model.AliasRenderInfo {
	slog.Debug("AliasManager: GetAliasesToRender started",
		"aliasMapSize", len(am.aliasMap),
		"requiredAliasesSize", len(am.requiredAliases))

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
			slog.Debug("AliasManager: skipping alias - type info not found",
				"fqn", fqn,
				"alias", alias)
			continue
		}
		aliasesToWrite = append(aliasesToWrite, aliasPair{alias, fqn})
	}

	slog.Debug("AliasManager: aliases to write",
		"count", len(aliasesToWrite))

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
		slog.Debug("AliasManager: alias to render",
			"alias", item.aliasName,
			"originalType", originalTypeName,
			"fqn", item.fqn,
			"typeInfoKind", typeInfo.Kind.String(),
			"typeInfoString", typeInfo.String())
	}

	return renderInfos
}

// 新增方法：设置字段类型信息
func (am *AliasManager) SetFieldTypesToAlias(fieldTypes map[string]*model.TypeInfo) {
	if am.fieldTypesToAlias == nil {
		am.fieldTypesToAlias = make(map[string]*model.TypeInfo)
	}
	for key, typ := range fieldTypes {
		am.fieldTypesToAlias[key] = typ
	}
}
