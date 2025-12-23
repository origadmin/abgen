package planner

import (
	"reflect"
	"testing"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/generator/components"
	"github.com/origadmin/abgen/internal/model"
)

// Helper function to create a struct type for tests
func newStruct(name, path string, fields []*model.FieldInfo) *model.TypeInfo {
	return &model.TypeInfo{
		Name:       name,
		ImportPath: path,
		Kind:       model.Struct,
		Fields:     fields,
	}
}

func TestPlanner_Plan(t *testing.T) {
	// --- Test Data Setup ---
	// Types
	sourceUser := newStruct("User", "source/ent", []*model.FieldInfo{
		{Name: "ID", Type: &model.TypeInfo{Name: "int", Kind: model.Primitive}},
		{Name: "Profile", Type: newStruct("Profile", "source/ent", nil)},
	})
	targetUser := newStruct("User", "target/dto", []*model.FieldInfo{
		{Name: "ID", Type: &model.TypeInfo{Name: "int", Kind: model.Primitive}},
		{Name: "Profile", Type: newStruct("Profile", "target/dto", nil)},
	})
	sourceProfile := newStruct("Profile", "source/ent", nil)
	targetProfile := newStruct("Profile", "target/dto", nil)

	typeInfos := map[string]*model.TypeInfo{
		"source/ent.User":    sourceUser,
		"source/ent.Profile": sourceProfile,
		"target/dto.User":    targetUser,
		"target/dto.Profile": targetProfile,
	}

	// Initial Config
	initialConfig := config.NewConfig()
	initialConfig.PackagePairs = append(initialConfig.PackagePairs, &config.PackagePair{
		SourcePath: "source/ent",
		TargetPath: "target/dto",
	})

	// --- Test Execution ---
	typeConverter := components.NewTypeConverter()
	planner := NewPlanner(typeConverter)
	plan := planner.Plan(initialConfig, typeInfos)

	// --- Assertions ---
	if plan == nil {
		t.Fatal("Plan() returned nil")
	}
	if plan.FinalConfig == nil {
		t.Fatal("ExecutionPlan has a nil FinalConfig")
	}

	// 1. Check for Disambiguation
	// Since source and target types share the name "User" and "Profile",
	// default suffixes should have been applied.
	if plan.FinalConfig.NamingRules.SourceSuffix != "Source" {
		t.Errorf("Expected SourceSuffix to be 'Source', got '%s'", plan.FinalConfig.NamingRules.SourceSuffix)
	}
	if plan.FinalConfig.NamingRules.TargetSuffix != "Target" {
		t.Errorf("Expected TargetSuffix to be 'Target', got '%s'", plan.FinalConfig.NamingRules.TargetSuffix)
	}

	// 2. Check Active Rules
	// We expect two rules: one for User -> User, and one for Profile -> Profile (discovered via dependency).
	if len(plan.ActiveRules) != 2 {
		t.Fatalf("Expected 2 active rules, got %d", len(plan.ActiveRules))
	}

	expectedRules := map[string]string{
		"source/ent.User":    "target/dto.User",
		"source/ent.Profile": "target/dto.Profile",
	}

	for _, rule := range plan.ActiveRules {
		target, ok := expectedRules[rule.SourceType]
		if !ok {
			t.Errorf("Found unexpected rule with source type: %s", rule.SourceType)
			continue
		}
		if target != rule.TargetType {
			t.Errorf("For source %s, expected target %s, but got %s", rule.SourceType, target, rule.TargetType)
		}
		// Remove from map to ensure we found all expected rules
		delete(expectedRules, rule.SourceType)
	}

	if len(expectedRules) > 0 {
		t.Errorf("Did not find all expected rules. Missing: %v", reflect.ValueOf(expectedRules).MapKeys())
	}
}
