package config

import (
	"reflect"
	"testing"
)

const mockCurrentPkgPath = "github.com/my/project/current"
const mockCurrentPkgName = "current"

func TestParser_Comprehensive(t *testing.T) {
	testCases := []struct {
		name           string
		directives     []string
		currentPkgPath string
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
			currentPkgPath: mockCurrentPkgPath,
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
				NamingRules: NamingRules{
					SourceSuffix: "Entity",
					TargetPrefix: "Proto",
				},
			},
		},
		{
			name: "Custom Func Rule Before Main Convert Rule (No PackagePath)",
			directives: []string{
				`//go:abgen:package:path=path/to/ent,alias=ent`,
				`//go:abgen:convert:rule="source:ent.Role,target:Role,func:ConvertRoleFunc"`,
				`//go:abgen:convert="source=ent.Role,target=Role"`,
			},
			currentPkgPath: "", // Simulate empty package path
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
			name: "Custom Func Rule Before Main Convert Rule (With PackagePath)",
			directives: []string{
				`//go:abgen:package:path=path/to/ent,alias=ent`,
				`//go:abgen:convert:rule="source:ent.Role,target:Role,func:ConvertRoleFunc"`,
				`//go:abgen:convert="source=ent.Role,target=Role"`,
			},
			currentPkgPath: mockCurrentPkgPath,
			expectedConfig: &Config{
				PackageAliases: map[string]string{
					"ent": "path/to/ent",
				},
				ConversionRules: []*ConversionRule{
					{
						SourceType: "path/to/ent.Role",
						TargetType: "github.com/my/project/current.Role",
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser()

			// The new ParseDirectives is robust enough to handle any order.
			cfg, err := p.ParseDirectives(tc.directives, mockCurrentPkgName, tc.currentPkgPath)
			if err != nil {
				t.Fatalf("ParseDirectives failed: %v", err)
			}

			// Normalize expected config for comparison
			if tc.expectedConfig.PackageAliases == nil {
				tc.expectedConfig.PackageAliases = make(map[string]string)
			}
			// Add default aliases to expected config for accurate comparison
			for alias, path := range defaultPackageAliases {
				if _, exists := tc.expectedConfig.PackageAliases[alias]; !exists {
					tc.expectedConfig.PackageAliases[alias] = path
				}
			}
			if tc.expectedConfig.ConversionRules == nil {
				tc.expectedConfig.ConversionRules = []*ConversionRule{}
			}
			if tc.expectedConfig.GlobalBehaviorRules.DefaultDirection == "" {
				tc.expectedConfig.GlobalBehaviorRules.DefaultDirection = DirectionBoth
			}

			// Compare PackageAliases
			if !reflect.DeepEqual(cfg.PackageAliases, tc.expectedConfig.PackageAliases) {
				t.Errorf("PackageAliases mismatch:\ngot:  %v\nwant: %v", cfg.PackageAliases, tc.expectedConfig.PackageAliases)
			}

			// Compare NamingRules
			if !reflect.DeepEqual(cfg.NamingRules, tc.expectedConfig.NamingRules) {
				t.Errorf("NamingRules mismatch:\ngot:  %+v\nwant: %+v", cfg.NamingRules, tc.expectedConfig.NamingRules)
			}

			// Compare ConversionRules
			if len(cfg.ConversionRules) != len(tc.expectedConfig.ConversionRules) {
				t.Fatalf("Expected %d conversion rules, got %d", len(tc.expectedConfig.ConversionRules), len(cfg.ConversionRules))
			}
			for i, rule := range cfg.ConversionRules {
				expectedRule := tc.expectedConfig.ConversionRules[i]
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
