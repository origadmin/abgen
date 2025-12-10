package generator

import (
	"testing"

	"github.com/origadmin/abgen/internal/analyzer"
	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

func TestGenerator_Integration(t *testing.T) {
	// Create a simple walker
	walker := analyzer.NewPackageWalker()
	
	// Create a rule set with a simple package pair
	ruleSet := config.NewRuleSet()
	ruleSet.PackagePairs["source/pkg"] = "target/pkg"
	
	// Create the generator
	generator := NewGenerator(walker, ruleSet)
	
	// Add a simple conversion task
	sourceType := &model.Type{
		Name:       "User",
		ImportPath: "source/pkg",
		Kind:       model.TypeKindStruct,
		Fields: []*model.Field{
			{
				Name: "ID",
				Type: &model.Type{
					Name: "int",
					Kind: model.TypeKindPrimitive,
				},
			},
			{
				Name: "Name",
				Type: &model.Type{
					Name: "string",
					Kind: model.TypeKindPrimitive,
				},
			},
		},
	}
	
	targetType := &model.Type{
		Name:       "UserDTO",
		ImportPath: "target/pkg",
		Kind:       model.TypeKindStruct,
		Fields: []*model.Field{
			{
				Name: "ID",
				Type: &model.Type{
					Name: "int",
					Kind: model.TypeKindPrimitive,
				},
			},
			{
				Name: "Name",
				Type: &model.Type{
					Name: "string",
					Kind: model.TypeKindPrimitive,
				},
			},
		},
	}
	
	generator.AddTask(sourceType, targetType)
	
	// Generate code
	code, err := generator.Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}
	
	// Check that generated code contains expected elements
	expectedFunction := "ConvertUserToUserDTO"
	if !contains(string(code), expectedFunction) {
		t.Errorf("Generated code should contain function '%s'", expectedFunction)
	}
	
	expectedPackage := "package generated"
	if !contains(string(code), expectedPackage) {
		t.Errorf("Generated code should contain '%s'", expectedPackage)
	}
	
	t.Logf("Generated code:\n%s", string(code))
}

func TestGenerator_BasicStructConversion(t *testing.T) {
	walker := analyzer.NewPackageWalker()
	ruleSet := config.NewRuleSet()
	generator := NewGenerator(walker, ruleSet)
	
	// Test basic struct conversion
	sourceType := &model.Type{
		Name: "User",
		Kind: model.TypeKindStruct,
		Fields: []*model.Field{
			{
				Name: "ID",
				Type: &model.Type{
					Name: "int",
					Kind: model.TypeKindPrimitive,
				},
			},
		},
	}
	
	targetType := &model.Type{
		Name: "UserDTO",
		Kind: model.TypeKindStruct,
		Fields: []*model.Field{
			{
				Name: "ID",
				Type: &model.Type{
					Name: "int",
					Kind: model.TypeKindPrimitive,
				},
			},
		},
	}
	
	generator.AddTask(sourceType, targetType)
	
	code, err := generator.Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}
	
	// Should contain struct conversion logic
	if !contains(string(code), "to := &UserDTO{}") {
		t.Error("Generated code should create target struct instance")
	}
	
	if !contains(string(code), "to.ID = from.ID") {
		t.Error("Generated code should assign field values")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}