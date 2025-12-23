package analyzer

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/origadmin/abgen/internal/config"
)

// TestTypeAnalyzer_Analyze_CoreParsing tests the analyzer's ability to correctly
// parse directives from a source file and produce the right config.
// It uses the test case in `../../testdata/00_core_parsing/rule_construction`.
func TestTypeAnalyzer_Analyze_CoreParsing(t *testing.T) {
	// Use a relative path to the testdata directory to keep the test portable.
	testDir, err := filepath.Abs("../../testdata/00_core_parsing/rule_construction")
	if err != nil {
		t.Fatalf("Failed to get absolute path for testdata: %v", err)
	}

	analyzer := NewTypeAnalyzer()
	cfg, _, err := analyzer.Analyze(testDir)

	// --- Assertions ---
	if err != nil {
		// We expect errors here because the packages (e.g., github.com/my/project/ent/v1) don't actually exist.
		// The key is that the config parsing part should still succeed.
		t.Logf("Analyze() returned an expected error because dummy packages don't exist: %v", err)
	}
	if cfg == nil {
		t.Fatal("Analyze() returned a nil config despite non-existent packages")
	}

	// 1. Verify the Config object based on source.go
	// Check package aliases - 'ent' should be overwritten by the last directive
	expectedEntPath := "github.com/my/project/ent/v2"
	if path, ok := cfg.PackageAliases["ent"]; !ok || path != expectedEntPath {
		t.Errorf("Expected alias 'ent' to map to '%s', got '%s'", expectedEntPath, path)
	}
	expectedPbPath := "github.com/my/project/pb"
	if path, ok := cfg.PackageAliases["pb"]; !ok || path != expectedPbPath {
		t.Errorf("Expected alias 'pb' to map to '%s', got '%s'", expectedPbPath, path)
	}

	// Check global naming rules
	if cfg.NamingRules.TargetSuffix != "Model" {
		t.Errorf("Expected NamingRules.TargetSuffix to be 'Model', got '%s'", cfg.NamingRules.TargetSuffix)
	}

	// Check package pairs
	if len(cfg.PackagePairs) != 2 {
		t.Fatalf("Expected 2 package pairs, got %d", len(cfg.PackagePairs))
	}
	expectedPair1 := &config.PackagePair{SourcePath: "github.com/my/project/ent/v2", TargetPath: "github.com/my/project/pb"}
	expectedPair2 := &config.PackagePair{SourcePath: "github.com/another/pkg", TargetPath: "github.com/my/project/pb"}
	if !reflect.DeepEqual(cfg.PackagePairs[0], expectedPair1) && !reflect.DeepEqual(cfg.PackagePairs[0], expectedPair2) {
		t.Errorf("Unexpected package pair found: %+v", cfg.PackagePairs[0])
	}
	if !reflect.DeepEqual(cfg.PackagePairs[1], expectedPair1) && !reflect.DeepEqual(cfg.PackagePairs[1], expectedPair2) {
		t.Errorf("Unexpected package pair found: %+v", cfg.PackagePairs[1])
	}

	// Check conversion rules
	if len(cfg.ConversionRules) != 2 {
		t.Fatalf("Expected 2 conversion rules, got %d", len(cfg.ConversionRules))
	}

	// Find and check the complex rule for User
	var userRule *config.ConversionRule
	for _, r := range cfg.ConversionRules {
		if strings.HasSuffix(r.SourceType, "ent/v2.User") {
			userRule = r
			break
		}
	}

	if userRule == nil {
		t.Fatal("Conversion rule for 'ent.User' not found")
	}
	if userRule.TargetType != "github.com/my/project/pb.User" {
		t.Errorf("Expected user rule target to be 'github.com/my/project/pb.User', got '%s'", userRule.TargetType)
	}
	if userRule.Direction != config.DirectionBoth {
		t.Errorf("Expected user rule direction to be 'both', got '%v'", userRule.Direction)
	}
	if _, ok := userRule.FieldRules.Ignore["PasswordHash"]; !ok {
		t.Error("Expected 'PasswordHash' to be in ignore map")
	}
	if _, ok := userRule.FieldRules.Ignore["Salt"]; !ok {
		t.Error("Expected 'Salt' to be in ignore map")
	}
	if val, ok := userRule.FieldRules.Remap["CreatedAt"]; !ok || val != "CreatedTimestamp" {
		t.Errorf("Expected remap 'CreatedAt' to be 'CreatedTimestamp', got '%s'", val)
	}

	// Check the simple rule for Data
	var dataRule *config.ConversionRule
	for _, r := range cfg.ConversionRules {
		if strings.HasSuffix(r.SourceType, "pkg.Data") {
			dataRule = r
			break
		}
	}
	if dataRule == nil {
		t.Fatal("Conversion rule for 'pkg.Data' not found")
	}
	if dataRule.TargetType != "github.com/my/project/pb.Data" {
		t.Errorf("Expected data rule target to be 'github.com/my/project/pb.Data', got '%s'", dataRule.TargetType)
	}
}
