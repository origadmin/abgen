package config

import (
	"go/ast"
	"reflect"
	"testing"

	"golang.org/x/tools/go/packages"
)

// mockPackage creates a minimal packages.Package for testing.
func mockPackage() *packages.Package {
	return &packages.Package{
		ID:   "github.com/my/project/current",
		Name: "current",
		Syntax: []*ast.File{
			{
				// This is a simplified AST. In a real scenario, you'd populate this
				// with actual parsed source code containing the directives.
				// For this test, the directives are passed directly to the parser.
			},
		},
	}
}

func TestParser_ParseDirectives_NamingRules(t *testing.T) {
	p := NewParser()
	// Mocking directives as if they were discovered from source files
	directives := []string{
		`//go:abgen:convert:source:suffix=Ent`,
		`//go:abgen:convert:target:suffix=PB`,
		`//go:abgen:convert:source:prefix=My`,
		`//go:abgen:convert:target:prefix=Api`,
		`//go:abgen:convert:source:suffix=Entity`, // Override
	}

	// We need to manually call the internal parse method for this unit test
	for _, d := range directives {
		p.parseSingleDirective(d)
	}

	cfg := p.config

	if cfg.NamingRules.SourceSuffix != "Entity" {
		t.Errorf("Expected SourceSuffix to be 'Entity', got '%s'", cfg.NamingRules.SourceSuffix)
	}
	if cfg.NamingRules.TargetSuffix != "PB" {
		t.Errorf("Expected TargetSuffix to be 'PB', got '%s'", cfg.NamingRules.TargetSuffix)
	}
	if cfg.NamingRules.SourcePrefix != "My" {
		t.Errorf("Expected SourcePrefix to be 'My', got '%s'", cfg.NamingRules.SourcePrefix)
	}
	if cfg.NamingRules.TargetPrefix != "Api" {
		t.Errorf("Expected TargetPrefix to be 'Api', got '%s'", cfg.NamingRules.TargetPrefix)
	}
}

func TestParser_PackageAlias_Override(t *testing.T) {
	p := NewParser()
	directives := []string{
		`//go:abgen:package:path=path/to/pkg1,alias=p`,
		`//go:abgen:package:path=path/to/pkg2,alias=p`, // This should override the previous one
	}

	for _, d := range directives {
		p.parseSingleDirective(d)
	}

	cfg := p.config
	expected := "path/to/pkg2"
	if cfg.PackageAliases["p"] != expected {
		t.Errorf("PackageAliases = %v, want %v", cfg.PackageAliases["p"], expected)
	}
}

func TestParser_ParseDirectives_CumulativePackagePairs(t *testing.T) {
	p := NewParser()
	directives := []string{
		`//go:abgen:package:path=path/to/source1,alias=s`,
		`//go:abgen:package:path=path/to/target1,alias=t`,
		`//go:abgen:pair:packages=s,t`,
		`//go:abgen:package:path=path/to/target2,alias=t2`,
		`//go:abgen:pair:packages=s,t2`,
	}

	for _, d := range directives {
		p.parseSingleDirective(d)
	}

	cfg := p.config
	expectedPairs := []*PackagePair{
		{SourcePath: "path/to/source1", TargetPath: "path/to/target1"},
		{SourcePath: "path/to/source1", TargetPath: "path/to/target2"},
	}

	if len(cfg.PackagePairs) != len(expectedPairs) {
		t.Fatalf("Expected %d package pairs, got %d", len(expectedPairs), len(cfg.PackagePairs))
	}

	// Create a map for easier comparison, as order doesn't matter.
	expectedMap := make(map[PackagePair]bool)
	for _, p := range expectedPairs {
		expectedMap[*p] = true
	}

	for _, p := range cfg.PackagePairs {
		if !expectedMap[*p] {
			t.Errorf("Unexpected package pair: %+v", *p)
		}
	}
}

func TestParser_ParseDirectives_ConversionRule(t *testing.T) {
	p := NewParser()
	directives := []string{
		`//go:abgen:package:path=path/to/ent,alias=ent`,
		`//go:abgen:convert="source=ent.User,target=User,direction=both,ignore=Password;Salt,remap=CreatedAt:Created"`,
	}

	for _, d := range directives {
		p.parseSingleDirective(d)
	}

	cfg := p.config

	if len(cfg.ConversionRules) != 1 {
		t.Fatalf("Expected 1 conversion rule, got %d", len(cfg.ConversionRules))
	}

	rule := cfg.ConversionRules[0]
	expectedRule := &ConversionRule{
		SourceType: "path/to/ent.User",
		TargetType: "User",
		Direction:  DirectionBoth,
		FieldRules: FieldRuleSet{
			Ignore: map[string]struct{}{"Password": {}, "Salt": {}},
			Remap:  map[string]string{"CreatedAt": "Created"},
		},
	}

	if rule.SourceType != expectedRule.SourceType {
		t.Errorf("SourceType = %s, want %s", rule.SourceType, expectedRule.SourceType)
	}
	if rule.TargetType != expectedRule.TargetType {
		t.Errorf("TargetType = %s, want %s", rule.TargetType, expectedRule.TargetType)
	}
	if rule.Direction != expectedRule.Direction {
		t.Errorf("Direction = %s, want %s", rule.Direction, expectedRule.Direction)
	}
	if !reflect.DeepEqual(rule.FieldRules.Ignore, expectedRule.FieldRules.Ignore) {
		t.Errorf("Ignore rules = %v, want %v", rule.FieldRules.Ignore, expectedRule.FieldRules.Ignore)
	}
	if !reflect.DeepEqual(rule.FieldRules.Remap, expectedRule.FieldRules.Remap) {
		t.Errorf("Remap rules = %v, want %v", rule.FieldRules.Remap, expectedRule.FieldRules.Remap)
	}
}

func TestParser_SupportsFullPath(t *testing.T) {
	p := NewParser()
	directives := []string{
		`//go:abgen:package:path=github.com/my/source,alias=s`,
		// Use a full path for the target in pair:packages
		`//go:abgen:pair:packages=s,github.com/my/target`,
		// Use a full path for the source in convert
		`//go:abgen:convert="source=github.com/another/source.Data,target=LocalData"`,
	}

	for _, d := range directives {
		p.parseSingleDirective(d)
	}

	cfg := p.config

	// Test pair:packages with full path
	if len(cfg.PackagePairs) != 1 {
		t.Fatalf("Expected 1 package pair, got %d", len(cfg.PackagePairs))
	}
	expectedPair := &PackagePair{SourcePath: "github.com/my/source", TargetPath: "github.com/my/target"}
	if *cfg.PackagePairs[0] != *expectedPair {
		t.Errorf("PackagePair = %+v, want %+v", *cfg.PackagePairs[0], *expectedPair)
	}

	// Test convert with full path
	if len(cfg.ConversionRules) != 1 {
		t.Fatalf("Expected 1 conversion rule, got %d", len(cfg.ConversionRules))
	}
	expectedRule := &ConversionRule{
		SourceType: "github.com/another/source.Data",
		TargetType: "LocalData",
		Direction:  DirectionOneway,
	}
	rule := cfg.ConversionRules[0]
	if rule.SourceType != expectedRule.SourceType {
		t.Errorf("SourceType = %s, want %s", rule.SourceType, expectedRule.SourceType)
	}
	if rule.TargetType != expectedRule.TargetType {
		t.Errorf("TargetType = %s, want %s", rule.TargetType, expectedRule.TargetType)
	}
}

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser() returned nil")
	}
	if p.config == nil {
		t.Fatal("NewParser().config is nil")
	}
}
