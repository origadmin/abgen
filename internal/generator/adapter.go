package generator

import (
	"fmt"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/generator/components"
	"github.com/origadmin/abgen/internal/model"
)

// LegacyGeneratorAdapter adapts the new CodeGenerator to the old generator interface for compatibility.
type LegacyGeneratorAdapter struct {
	orchestrator model.CodeGenerator
	config       *config.Config
}

// NewLegacyGeneratorAdapter creates an adapter for the legacy generator.
func NewLegacyGeneratorAdapter(config *config.Config) *LegacyGeneratorAdapter {
	return &LegacyGeneratorAdapter{
		orchestrator: NewCodeGenerator(), // Updated: No arguments
		config:       config,
	}
}

// Generate uses the new architecture to generate code but conforms to an older signature.
func (l *LegacyGeneratorAdapter) Generate(typeInfos map[string]*model.TypeInfo) ([]byte, error) {
	// Updated: Call the new Generate method directly.
	response, err := l.orchestrator.Generate(l.config, typeInfos)
	if err != nil {
		return nil, err
	}

	return response.GeneratedCode, nil
}

// ComponentFactory is a helper for creating individual components for testing.
type ComponentFactory struct{}

// NewComponentFactory creates a new component factory.
func NewComponentFactory() *ComponentFactory {
	return &ComponentFactory{}
}

// CreateTypeConverter creates a type converter component.
func (f *ComponentFactory) CreateTypeConverter() model.TypeConverter {
	return components.NewTypeConverter()
}

// CreateImportManager creates an import manager component.
func (f *ComponentFactory) CreateImportManager() model.ImportManager {
	return components.NewImportManager()
}

// CreateNameGenerator creates a name generator component.
func (f *ComponentFactory) CreateNameGenerator(config *config.Config, importManager model.ImportManager) model.NameGenerator {
	// Updated: Correct signature.
	return components.NewNameGenerator(config, importManager)
}

// CreateAliasManager creates an alias manager component.
func (f *ComponentFactory) CreateAliasManager(
	config *config.Config,
	importManager model.ImportManager,
	nameGenerator model.NameGenerator,
	typeInfos map[string]*model.TypeInfo,
) model.AliasManager {
	// Updated: Correct signature.
	return components.NewAliasManager(config, importManager, nameGenerator, typeInfos)
}

// CreateConversionEngine creates a conversion engine component.
func (f *ComponentFactory) CreateConversionEngine(
	typeConverter model.TypeConverter,
	nameGenerator model.NameGenerator,
	aliasManager model.AliasManager,
	importManager model.ImportManager,
) model.ConversionEngine {
	return components.NewConversionEngine(typeConverter, nameGenerator, aliasManager, importManager)
}

// CreateCodeEmitter creates a code emitter component.
func (f *ComponentFactory) CreateCodeEmitter(
	config *config.Config,
	importManager model.ImportManager,
	aliasManager model.AliasManager,
) model.CodeEmitter {
	return components.NewCodeEmitter(config, importManager, aliasManager)
}

// CreateCodeGenerator creates a code generator component.
func (f *ComponentFactory) CreateCodeGenerator() model.CodeGenerator {
	// Updated: No arguments.
	return NewCodeGenerator()
}

// MigrationHelper helps migrate from the old architecture to the new one.
type MigrationHelper struct{}

// NewMigrationHelper creates a new migration helper.
func NewMigrationHelper() *MigrationHelper {
	return &MigrationHelper{}
}

// MigrateOldGenerator migrates an old generator to the new architecture.
func (m *MigrationHelper) MigrateOldGenerator(oldGen *LegacyGenerator) model.CodeGenerator {
	if oldGen == nil {
		return nil
	}

	factory := NewComponentFactory()
	orchestrator := factory.CreateCodeGenerator()

	fmt.Printf("Migrated old Generator to new orchestrator-based architecture\n")

	return orchestrator
}

// ValidateMigration validates if the migration was successful.
func (m *MigrationHelper) ValidateMigration(oldGen *LegacyGenerator, newGen model.CodeGenerator) error {
	if oldGen == nil && newGen == nil {
		return fmt.Errorf("both generators are nil")
	}
	if newGen == nil {
		return fmt.Errorf("new generator is nil")
	}
	return nil
}
