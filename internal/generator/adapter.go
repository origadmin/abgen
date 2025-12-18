package generator

import (
	"fmt"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/generator/components"
	"github.com/origadmin/abgen/internal/model"
)

// LegacyGeneratorAdapter 适配器，让旧的 Generator 接口可以访问新组件的功能
type LegacyGeneratorAdapter struct {
	orchestrator model.CodeGenerator
	config       *config.Config
}

// NewLegacyGeneratorAdapter 创建旧的生成器适配器
func NewLegacyGeneratorAdapter(config *config.Config, typeInfos map[string]*model.TypeInfo) *LegacyGeneratorAdapter {
	return &LegacyGeneratorAdapter{
		orchestrator: NewOrchestrator(config, typeInfos),
		config:       config,
	}
}

// Generate 使用新架构生成代码，但返回旧的格式
func (l *LegacyGeneratorAdapter) Generate(typeInfos map[string]*model.TypeInfo) ([]byte, error) {
	request := &model.GenerationRequest{
		Context: &model.GenerationContext{
			Config:           l.config,
			TypeInfos:        typeInfos,
			InvolvedPackages: make(map[string]struct{}),
		},
	}

	response, err := l.orchestrator.Generate(request)
	if err != nil {
		return nil, err
	}

	return response.GeneratedCode, nil
}

// GetComponentFactory 创建组件工厂，用于测试和特定场景
type ComponentFactory struct{}

// NewComponentFactory 创建组件工厂
func NewComponentFactory() *ComponentFactory {
	return &ComponentFactory{}
}

// CreateTypeConverter 创建类型转换器组件
func (f *ComponentFactory) CreateTypeConverter() model.TypeConverter {
	return components.NewTypeConverter()
}

// CreateImportManager 创建导入管理器组件
func (f *ComponentFactory) CreateImportManager() model.ImportManager {
	return components.NewImportManager()
}

// CreateNameGenerator 创建命名生成器组件
func (f *ComponentFactory) CreateNameGenerator(config *config.Config, aliasMap map[string]string) model.NameGenerator {
	return components.NewNameGenerator(config, aliasMap)
}

// CreateAliasManager 创建别名管理器组件
func (f *ComponentFactory) CreateAliasManager(
	config *config.Config,
	nameGenerator model.NameGenerator,
	typeInfos map[string]*model.TypeInfo,
) model.AliasManager {
	return components.NewAliasManager(config, nameGenerator, typeInfos)
}

// CreateConversionEngine 创建转换引擎组件
func (f *ComponentFactory) CreateConversionEngine(
	typeConverter model.TypeConverter,
	nameGenerator model.NameGenerator,
	aliasManager model.AliasManager,
	importManager model.ImportManager,
) model.ConversionEngine {
	return components.NewConversionEngine(typeConverter, nameGenerator, aliasManager, importManager)
}

// CreateCodeEmitter 创建代码发射器组件
func (f *ComponentFactory) CreateCodeEmitter(
	config *config.Config,
	importManager model.ImportManager,
	aliasManager model.AliasManager,
	nameGenerator model.NameGenerator,
	conversionEngine model.ConversionEngine,
	typeInfos map[string]*model.TypeInfo,
) model.CodeEmitter {
	return components.NewCodeEmitter(config, importManager, aliasManager, nameGenerator, conversionEngine, typeInfos)
}

// CreateOrchestrator 创建协调器组件
func (f *ComponentFactory) CreateOrchestrator(config *config.Config, typeInfos map[string]*model.TypeInfo) model.CodeGenerator {
	return NewOrchestrator(config, typeInfos)
}

// MigrationHelper 迁移助手，帮助从旧架构迁移到新架构
type MigrationHelper struct{}

// NewMigrationHelper 创建迁移助手
func NewMigrationHelper() *MigrationHelper {
	return &MigrationHelper{}
}

// MigrateOldGenerator 迁移旧的生成器到新架构
func (m *MigrationHelper) MigrateOldGenerator(oldGen *Generator) model.CodeGenerator {
	if oldGen == nil {
		return nil
	}

	// 创建新的组件
	factory := NewComponentFactory()
	
	// 创建别名映射
	aliasMap := make(map[string]string)
	for k, v := range oldGen.aliasMap {
		aliasMap[k] = v
	}

	// 创建协调器
	orchestrator := factory.CreateOrchestrator(oldGen.config, oldGen.typeInfos)

	// 记录迁移信息
	fmt.Printf("Migrated old Generator to new orchestrator-based architecture\n")

	return orchestrator
}

// ValidateMigration 验证迁移是否成功
func (m *MigrationHelper) ValidateMigration(oldGen *Generator, newGen model.CodeGenerator) error {
	if oldGen == nil && newGen == nil {
		return fmt.Errorf("both generators are nil")
	}
	
	if oldGen == nil && newGen != nil {
		return nil // 这是有效的，创建了新的生成器
	}
	
	if newGen == nil {
		return fmt.Errorf("new generator is nil")
	}

	// 这里可以添加更多的验证逻辑
	return nil
}