package components

import (
	"log/slog"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// ConcreteAliasManager 实现 AliasManager 接口
type ConcreteAliasManager struct {
	config           *config.Config
	nameGenerator    model.NameGenerator
	aliasMap         map[string]string
	requiredAliases  map[string]struct{}
	typeInfos        map[string]*model.TypeInfo
}

// NewAliasManager 创建新的别名管理器
func NewAliasManager(
	config *config.Config,
	nameGenerator model.NameGenerator,
	typeInfos map[string]*model.TypeInfo,
) model.AliasManager {
	return &ConcreteAliasManager{
		config:          config,
		nameGenerator:   nameGenerator,
		aliasMap:        make(map[string]string),
		requiredAliases: make(map[string]struct{}),
		typeInfos:       typeInfos,
	}
}

// EnsureTypeAlias 确保指定类型有别名，如果没有则创建
func (am *ConcreteAliasManager) EnsureTypeAlias(typeInfo *model.TypeInfo, isSource bool) {
	if typeInfo == nil {
		return
	}

	uniqueKey := typeInfo.UniqueKey()

	// 如果别名已存在，直接返回
	if _, exists := am.aliasMap[uniqueKey]; exists {
		return
	}

	// 创建新别名
	alias := am.nameGenerator.GetAlias(typeInfo, isSource)

	// 立即存储到映射中
	am.aliasMap[uniqueKey] = alias
	am.requiredAliases[uniqueKey] = struct{}{}

	slog.Debug("AliasManager: ensured type alias",
		"type", typeInfo.String(),
		"uniqueKey", uniqueKey,
		"alias", alias,
		"isSource", isSource)

	// 递归处理复合类型的元素类型
	am.ensureElementTypeAlias(typeInfo, isSource)
}

// ensureElementTypeAlias 递归确保元素类型的别名
func (am *ConcreteAliasManager) ensureElementTypeAlias(typeInfo *model.TypeInfo, isSource bool) {
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
		// 为命名的结构体字段类型递归创建别名
		for _, field := range typeInfo.Fields {
			am.EnsureTypeAlias(field.Type, isSource)
		}
	}
}

// GetAliasMap 返回别名映射
func (am *ConcreteAliasManager) GetAliasMap() map[string]string {
	return am.aliasMap
}

// GetRequiredAliases 返回需要的别名集合
func (am *ConcreteAliasManager) GetRequiredAliases() map[string]struct{} {
	return am.requiredAliases
}

// PopulateAliases 填充所有转换规则的别名
func (am *ConcreteAliasManager) PopulateAliases() {
	for _, rule := range am.config.ConversionRules {
		sourceInfo := am.typeInfos[rule.SourceType]
		targetInfo := am.typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}

		// 确保基础类型有别名
		am.EnsureTypeAlias(sourceInfo, true)
		am.EnsureTypeAlias(targetInfo, false)
	}
}