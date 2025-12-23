package components

import (
	"go/types"
	"strings"
	"testing"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// mockImportManager is a simple mock for testing AliasManager.
type mockImportManager struct {
	aliases map[string]string
}

func (m *mockImportManager) Add(pkgPath string) string {
	alias := pkgPath[strings.LastIndex(pkgPath, "/")+1:]
	m.aliases[pkgPath] = alias
	return alias
}
func (m *mockImportManager) AddAs(pkgPath, alias string) string {
	m.aliases[pkgPath] = alias
	return alias
}
func (m *mockImportManager) GetAlias(pkgPath string) (string, bool) {
	alias, ok := m.aliases[pkgPath]
	return alias, ok
}
func (m *mockImportManager) GetAllImports() map[string]string { return m.aliases }
func (m *mockImportManager) PackageName(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	if alias, ok := m.aliases[pkg.Path()]; ok {
		return alias
	}
	return pkg.Name()
}

func TestAliasManager_PopulateAliases(t *testing.T) {
	// --- Test Data Setup ---
	sourceUserType := newStruct("User", "source/ent", []*model.FieldInfo{
		{Name: "ID", Type: newPrimitive("int")},
		{Name: "Profile", Type: newStruct("Profile", "source/ent", nil)},
	})
	targetUserType := newStruct("User", "target/dto", []*model.FieldInfo{
		{Name: "ID", Type: newPrimitive("int")},
		{Name: "Profile", Type: newStruct("Profile", "target/dto", nil)},
	})
	timeType := newStruct("Time", "time", nil)

	// Build the AnalysisResult that will be the single source of truth
	analysisResult := &model.AnalysisResult{
		TypeInfos: map[string]*model.TypeInfo{
			"source/ent.User":    sourceUserType,
			"source/ent.Profile": newStruct("Profile", "source/ent", nil),
			"target/dto.User":    targetUserType,
			"target/dto.Profile": newStruct("Profile", "target/dto", nil),
			"time.Time":          timeType,
		},
		ExistingAliases: map[string]string{
			"UserProfileDTO": "target/dto.Profile",
		},
		ExecutionPlan: &model.ExecutionPlan{
			FinalConfig: config.NewConfig(),
		},
	}
	cfg := analysisResult.ExecutionPlan.FinalConfig
	cfg.ConversionRules = append(cfg.ConversionRules, &config.ConversionRule{
		SourceType: "source/ent.User",
		TargetType: "target/dto.User",
	})
	cfg.NamingRules.SourceSuffix = "Source"
	cfg.NamingRules.TargetSuffix = "DTO"

	// --- Test Execution ---
	im := &mockImportManager{aliases: make(map[string]string)}
	am := NewAliasManager(analysisResult, im)
	am.PopulateAliases()

	// --- Assertions ---
	expectedAliases := map[string]string{
		"source/ent.User":    "UserSource",
		"source/ent.Profile": "ProfileSource",
		"target/dto.User":    "UserDTO",
		"target/dto.Profile": "UserProfileDTO",
	}

	for key, expectedAlias := range expectedAliases {
		alias, ok := am.LookupAlias(key)
		if !ok {
			t.Errorf("Expected alias for key '%s', but none was found", key)
			continue
		}
		if alias != expectedAlias {
			t.Errorf("For key '%s', expected alias '%s', but got '%s'", key, expectedAlias, alias)
		}
	}

	if _, ok := am.LookupAlias("time.Time"); ok {
		t.Error("Expected 'time.Time' not to have an alias, but it did")
	}

	aliasedToGenerate := am.GetAliasedTypes()
	expectedToGenerate := map[string]struct{}{
		"source/ent.User":    {},
		"source/ent.Profile": {},
		"target/dto.User":    {},
	}

	if len(aliasedToGenerate) != len(expectedToGenerate) {
		t.Errorf("Expected %d types for alias generation, got %d", len(expectedToGenerate), len(aliasedToGenerate))
	}

	for key := range expectedToGenerate {
		if _, ok := aliasedToGenerate[key]; !ok {
			t.Errorf("Expected type '%s' to be in the set of generated aliases", key)
		}
	}

	if _, ok := aliasedToGenerate["target/dto.Profile"]; ok {
		t.Error("User-defined alias 'target/dto.Profile' should not be in the set of generated aliases")
	}
}
