package analyzer

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/origadmin/abgen/internal/config"
)

// TestTypeAnalyzer_Analyze_CoreParsing tests the analyzer's ability to correctly
// parse directives from a source file and produce a valid execution plan.
func TestTypeAnalyzer_Analyze_CoreParsing(t *testing.T) {
	testDir, err := filepath.Abs("../../testdata/00_core_parsing/rule_construction")
	if err != nil {
		t.Fatalf("Failed to get absolute path for testdata: %v", err)
	}

	analyzer := NewTypeAnalyzer()
	// In this test, we expect an error because the referenced packages don't exist.
	// However, the initial parsing of the config should still be part of the result,
	// even if the full analysis fails later. The new Analyze implementation returns
	// the result object even on partial failure.
	analysisResult, err := analyzer.Analyze(testDir)

	// We expect an error, but we also expect a non-nil result containing the parsed config.
	if err == nil {
		t.Logf("Warning: Analyze() did not return an error, which was expected because dummy packages don't exist.")
	}
	if analysisResult == nil {
		t.Fatalf("Analyze() returned a nil result, even on partial success.")
	}
	if analysisResult.ExecutionPlan == nil {
		t.Fatalf("Analyze() returned a result with a nil execution plan.")
	}
	if analysisResult.ExecutionPlan.FinalConfig == nil {
		t.Fatalf("Execution plan has a nil config.")
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
	// Since type analysis fails, the rule expansion might not be complete,
	// but we should at least have the explicitly defined rules.
	activeRules := analysisResult.ExecutionPlan.ActiveRules
	if len(activeRules) < 2 {
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
