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

func TestParser_Comprehensive(t *testing.T) {
	testCases := []struct {
		name           string
		directives     []string
		expectedConfig *Config
	}{
		{
			name: "Full Conversion Rule with Custom Func",
			directives: []string{
				`//go:abgen:package:path=path/to/ent,alias=ent`,
				`//go:abgen:package:path=path/to/pb,alias=pb`,
				`//go:abgen:convert="source=ent.User,target=pb.User,direction=both,ignore=Password;Salt,remap=CreatedAt:Created"`,
				`//go:abgen:convert:rule="source:ent.User,target:pb.User,func:ConvertUserWithPermissions"`,
				`//go:abgen:convert:source:suffix=Entity`,
				`//go:abgen:convert:target:prefix=Proto`,
			},
			expectedConfig: &Config{
				PackageAliases: map[string]string{
					"ent": "path/to/ent",
					"pb":  "path/to/pb",
				},
				ConversionRules: []*ConversionRule{
					{
						SourceType: "path/to/ent.User",
						TargetType: "path/to/pb.User",
						Direction:  DirectionBoth,
						FieldRules: FieldRuleSet{
							Ignore: map[string]struct{}{"Password": {}, "Salt": {}},
							Remap:  map[string]string{"CreatedAt": "Created"},
						},
						CustomFunc: "ConvertUserWithPermissions",
					},
				},
				NamingRules: NamingRule{
					SourceSuffix: "Entity",
					TargetPrefix: "Proto",
				},
			},
		},
		{
			name: "Custom Func Rule Before Main Convert Rule",
			directives: []string{
				`//go:abgen:package:path=path/to/ent,alias=ent`,
				`//go:abgen:convert:rule="source:ent.Role,target:Role,func:ConvertRoleFunc"`,
				`//go:abgen:convert="source=ent.Role,target=Role"`,
			},
			expectedConfig: &Config{
				PackageAliases: map[string]string{
					"ent": "path/to/ent",
				},
				ConversionRules: []*ConversionRule{
					{
						SourceType: "path/to/ent.Role",
						TargetType: "Role",
						Direction:  DirectionBoth,
						CustomFunc: "ConvertRoleFunc",
						FieldRules: FieldRuleSet{
							Ignore: make(map[string]struct{}),
							Remap:  make(map[string]string),
						},
					},
				},
			},
		},
		{
			name: "Package Pairs and Full Paths",
			directives: []string{
				`//go:abgen:package:path=github.com/my/source,alias=s`,
				`//go:abgen:pair:packages=s,github.com/my/target`,
			},
			expectedConfig: &Config{
				PackageAliases: map[string]string{
					"s": "github.com/my/source",
				},
				PackagePairs: []*PackagePair{
					{SourcePath: "github.com/my/source", TargetPath: "github.com/my/target"},
				},
			},
		},
		{
			name: "Global Behavior Rules",
			directives: []string{
				`//go:abgen:convert:alias:generate=true`,
			},
			expectedConfig: &Config{
				GlobalBehaviorRules: BehaviorRule{
					GenerateAlias: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser()

			for _, d := range tc.directives {
				if err := p.parseSingleDirective(d); err != nil {
					t.Fatalf("parseSingleDirective failed: %v", err)
				}
			}

			// *** FIX ***: Manually call the merge step, as the test bypasses parseDirectives.
			p.mergeCustomFuncRules()

			cfg := p.config

			// Normalize expected config for comparison
			if tc.expectedConfig.PackageAliases == nil {
				tc.expectedConfig.PackageAliases = make(map[string]string)
			}
			if tc.expectedConfig.ConversionRules == nil {
				tc.expectedConfig.ConversionRules = []*ConversionRule{}
			}
			if tc.expectedConfig.PackagePairs == nil {
				tc.expectedConfig.PackagePairs = []*PackagePair{}
			}
			if tc.expectedConfig.CustomFunctionRules == nil {
				tc.expectedConfig.CustomFunctionRules = make(map[string]string)
			}

			// Check that all expected aliases from directives are present.
			// This avoids test brittleness if default aliases are added to NewParser.
			for alias, path := range tc.expectedConfig.PackageAliases {
				if gotPath, ok := cfg.PackageAliases[alias]; !ok || gotPath != path {
					t.Errorf("PackageAliases mismatch for alias '%s': got '%s', want '%s'", alias, gotPath, path)
				}
			}

			// Compare NamingRules
			if !reflect.DeepEqual(cfg.NamingRules, tc.expectedConfig.NamingRules) {
				t.Errorf("NamingRules mismatch:\ngot:  %+v\nwant: %+v", cfg.NamingRules, tc.expectedConfig.NamingRules)
			}

			// Compare GlobalBehaviorRules
			if !reflect.DeepEqual(cfg.GlobalBehaviorRules, tc.expectedConfig.GlobalBehaviorRules) {
				t.Errorf("GlobalBehaviorRules mismatch:\ngot:  %+v\nwant: %+v", cfg.GlobalBehaviorRules, tc.expectedConfig.GlobalBehaviorRules)
			}

			// Compare PackagePairs
			if !reflect.DeepEqual(cfg.PackagePairs, tc.expectedConfig.PackagePairs) {
				t.Errorf("PackagePairs mismatch:\ngot:  %v\nwant: %v", cfg.PackagePairs, tc.expectedConfig.PackagePairs)
			}

			// Compare ConversionRules
			if len(cfg.ConversionRules) != len(tc.expectedConfig.ConversionRules) {
				t.Fatalf("Expected %d conversion rules, got %d", len(tc.expectedConfig.ConversionRules), len(cfg.ConversionRules))
			}
			for i, rule := range cfg.ConversionRules {
				expectedRule := tc.expectedConfig.ConversionRules[i]
				// Normalize FieldRules for comparison
				if rule.FieldRules.Ignore == nil {
					rule.FieldRules.Ignore = make(map[string]struct{})
				}
				if rule.FieldRules.Remap == nil {
					rule.FieldRules.Remap = make(map[string]string)
				}
				if !reflect.DeepEqual(rule, expectedRule) {
					t.Errorf("ConversionRule at index %d mismatch:\ngot:  %+v\nwant: %+v", i, rule, expectedRule)
				}
			}
		})
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
