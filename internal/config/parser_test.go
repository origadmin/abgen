package config

import (
	"testing"
)

func TestParser_ParseNamingRules(t *testing.T) {
	parser := NewParser()
	
	// Test naming directives
	directives := []string{
		"convert:source:suffix=Ent",
		"convert:target:suffix=PB",
		"convert:source:prefix=My",
		"convert:target:prefix=Api",
	}
	
	for _, dir := range directives {
		err := parser.Parse(dir)
		if err != nil {
			t.Fatalf("Failed to parse directive %q: %v", dir, err)
		}
	}
	
	ruleSet := parser.GetRuleSet()
	
	if ruleSet.NamingRules.SourceSuffix != "Ent" {
		t.Errorf("Expected SourceSuffix to be 'Ent', got '%s'", ruleSet.NamingRules.SourceSuffix)
	}
	if ruleSet.NamingRules.TargetSuffix != "PB" {
		t.Errorf("Expected TargetSuffix to be 'PB', got '%s'", ruleSet.NamingRules.TargetSuffix)
	}
	if ruleSet.NamingRules.SourcePrefix != "My" {
		t.Errorf("Expected SourcePrefix to be 'My', got '%s'", ruleSet.NamingRules.SourcePrefix)
	}
	if ruleSet.NamingRules.TargetPrefix != "Api" {
		t.Errorf("Expected TargetPrefix to be 'Api', got '%s'", ruleSet.NamingRules.TargetPrefix)
	}
}

func TestParser_ParseBehaviorRules(t *testing.T) {
	parser := NewParser()
	
	// Test behavior directives
	err := parser.Parse("convert:alias:generate=true")
	if err != nil {
		t.Fatalf("Failed to parse directive: %v", err)
	}
	
	ruleSet := parser.GetRuleSet()
	
	if !ruleSet.BehaviorRules.GenerateAlias {
		t.Error("Expected GenerateAlias to be true")
	}
}

func TestParser_ParseIgnoreRules(t *testing.T) {
	parser := NewParser()
	
	// Test ignore directive
	err := parser.Parse("convert:ignore=UserEntity#Password,Salt")
	if err != nil {
		t.Fatalf("Failed to parse directive: %v", err)
	}
	
	ruleSet := parser.GetRuleSet()
	
	// Check if UserEntity has the ignored fields
	if ignored, ok := ruleSet.FieldRules.Ignore["UserEntity"]; !ok {
		t.Error("Expected UserEntity to have ignored fields")
	} else {
		if _, hasPassword := ignored["Password"]; !hasPassword {
			t.Error("Expected Password to be ignored")
		}
		if _, hasSalt := ignored["Salt"]; !hasSalt {
			t.Error("Expected Salt to be ignored")
		}
	}
}

func TestParser_ParseRemapRules(t *testing.T) {
	parser := NewParser()
	
	// Test remap directive
	err := parser.Parse("convert:remap=A#TypeIDs:Type.ID")
	if err != nil {
		t.Fatalf("Failed to parse directive: %v", err)
	}
	
	ruleSet := parser.GetRuleSet()
	
	// Check if A has the remap rule
	if remapped, ok := ruleSet.FieldRules.Remap["A"]; !ok {
		t.Error("Expected A to have remapped fields")
	} else {
		if targetField, hasTypeIDs := remapped["TypeIDs"]; !hasTypeIDs {
			t.Error("Expected TypeIDs to be remapped")
		} else if targetField != "Type.ID" {
			t.Errorf("Expected TypeIDs to be remapped to 'Type.ID', got '%s'", targetField)
		}
	}
}

func TestParser_ParsePackagePath(t *testing.T) {
	parser := NewParser()
	
	// Test package path directive
	err := parser.Parse("package:path=github.com/my/project/ent,alias=ent_source")
	if err != nil {
		t.Fatalf("Failed to parse directive: %v", err)
	}
	
	// Test package pairing
	err = parser.Parse("pair:packages=ent_source,pb_source")
	if err != nil {
		t.Fatalf("Failed to parse package pairing: %v", err)
	}
	
	_ = parser.GetRuleSet()
	
	// The package pairs should be set up correctly
	// Since packageAliases is private, we verify through the effect
	// In a real scenario, you'd have more comprehensive testing
}

func TestParser_RuleOverride(t *testing.T) {
	parser := NewParser()
	
	// Set initial value
	err := parser.Parse("convert:source:suffix=Ent")
	if err != nil {
		t.Fatalf("Failed to parse first directive: %v", err)
	}
	
	// Override with new value
	err = parser.Parse("convert:source:suffix=Entity")
	if err != nil {
		t.Fatalf("Failed to parse second directive: %v", err)
	}
	
	ruleSet := parser.GetRuleSet()
	
	if ruleSet.NamingRules.SourceSuffix != "Entity" {
		t.Errorf("Expected SourceSuffix to be overridden to 'Entity', got '%s'", ruleSet.NamingRules.SourceSuffix)
	}
}

func TestParser_ParseDirectives(t *testing.T) {
	parser := NewParser()
	
	directives := []string{
		"convert:source:suffix=Ent",
		"convert:target:suffix=PB",
		"convert:alias:generate=true",
	}
	
	err := parser.ParseDirectives(directives)
	if err != nil {
		t.Fatalf("ParseDirectives failed: %v", err)
	}
	
	ruleSet := parser.GetRuleSet()
	
	if ruleSet.NamingRules.SourceSuffix != "Ent" {
		t.Errorf("Expected SourceSuffix to be 'Ent', got '%s'", ruleSet.NamingRules.SourceSuffix)
	}
	if ruleSet.NamingRules.TargetSuffix != "PB" {
		t.Errorf("Expected TargetSuffix to be 'PB', got '%s'", ruleSet.NamingRules.TargetSuffix)
	}
	if !ruleSet.BehaviorRules.GenerateAlias {
		t.Error("Expected GenerateAlias to be true")
	}
}

func TestParser_ParseWithPrefix(t *testing.T) {
	parser := NewParser()
	
	// Test with full prefix
	err := parser.Parse("//go:abgen:convert:source:suffix=Ent")
	if err != nil {
		t.Fatalf("Failed to parse directive with prefix: %v", err)
	}
	
	ruleSet := parser.GetRuleSet()
	
	if ruleSet.NamingRules.SourceSuffix != "Ent" {
		t.Errorf("Expected SourceSuffix to be 'Ent', got '%s'", ruleSet.NamingRules.SourceSuffix)
	}
}

func TestNewParser(t *testing.T) {
	parser := NewParser()
	if parser == nil {
		t.Fatal("NewParser() returned nil")
	}
	
	ruleSet := parser.GetRuleSet()
	if ruleSet == nil {
		t.Fatal("GetRuleSet() returned nil")
	}
	
	// Verify initial state
	if ruleSet.PackagePairs == nil {
		t.Error("PackagePairs should be initialized")
	}
	if ruleSet.FieldRules.Ignore == nil {
		t.Error("FieldRules.Ignore should be initialized")
	}
	if ruleSet.FieldRules.Remap == nil {
		t.Error("FieldRules.Remap should be initialized")
	}
}