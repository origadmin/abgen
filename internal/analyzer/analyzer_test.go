package analyzer

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/origadmin/abgen/internal/config"
)

// TestTypeAnalyzer_Analyze_CoreParsing tests the analyzer's ability to correctly
// parse directives and produce a valid execution plan.
func TestTypeAnalyzer_Analyze_CoreParsing(t *testing.T) {
	testDir, err := filepath.Abs("../../testdata/00_core_parsing/rule_construction")
	if err != nil {
		t.Fatalf("Failed to get absolute path for testdata: %v", err)
	}

	analyzer := NewTypeAnalyzer()
	analysisResult, err := analyzer.Analyze(testDir)

	if err != nil {
		t.Logf("Analyze() returned an expected error because dummy packages don't exist: %v", err)
	}
	if analysisResult == nil {
		t.Fatal("Analyze() returned a nil result")
	}
	if analysisResult.ExecutionPlan == nil {
		t.Fatal("Analyze() returned a result with a nil execution plan")
	}
	if analysisResult.ExecutionPlan.FinalConfig == nil {
		t.Fatal("Execution plan has a nil config")
	}

	cfg := analysisResult.ExecutionPlan.FinalConfig

	// Verify the Config object
	expectedEntPath := "github.com/my/project/ent/v2"
	if path, ok := cfg.PackageAliases["ent"]; !ok || path != expectedEntPath {
		t.Errorf("Expected alias 'ent' to map to '%s', got '%s'", expectedEntPath, path)
	}
	expectedPbPath := "github.com/my/project/pb"
	if path, ok := cfg.PackageAliases["pb"]; !ok || path != expectedPbPath {
		t.Errorf("Expected alias 'pb' to map to '%s', got '%s'", expectedPbPath, path)
	}

	if cfg.NamingRules.TargetSuffix != "Model" {
		t.Errorf("Expected NamingRules.TargetSuffix to be 'Model', got '%s'", cfg.NamingRules.TargetSuffix)
	}

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

	// Verify the active rules in the execution plan
	activeRules := analysisResult.ExecutionPlan.ActiveRules
	if len(activeRules) < 2 { // It might discover more, but at least the 2 explicit ones
		t.Fatalf("Expected at least 2 active rules, got %d", len(activeRules))
	}

	var userRule *config.ConversionRule
	for _, r := range activeRules {
		if strings.HasSuffix(r.SourceType, "ent/v2.User") {
			userRule = r
			break
		}
	}

	if userRule == nil {
		t.Fatal("Conversion rule for 'ent.User' not found in active rules")
	}
	if userRule.TargetType != "github.com/my/project/pb.User" {
		t.Errorf("Expected user rule target to be 'github.com/my/project/pb.User', got '%s'", userRule.TargetType)
	}
}
