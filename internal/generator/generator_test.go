package generator

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/origadmin/abgen/internal/analyzer"
	"github.com/origadmin/abgen/internal/config"
)

func init() {
	// Configure slog to debug mode for detailed testing information
	slog.SetDefault(slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	))
}

// Base dependencies for most tests
var baseDependencies = []string{
	"github.com/origadmin/abgen/testdata/fixtures/ent",
	"github.com/origadmin/abgen/testdata/fixtures/types",
}

var testCases = []struct {
	name           string
	directivePath  string
	goldenFileName string
	dependencies   []string
	priority       string
	category       string
	assertFunc     func(t *testing.T, generatedCode []byte, stubCode []byte)
}{
	// === 02_basic_conversions: Basic Struct Conversion ===
	{
		name:          "simple_struct_conversion",
		directivePath: "../../testdata/02_basic_conversions/simple_struct",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/source",
			"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/target",
		},
		priority: "P0",
		category: "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `UserName:\s+from.Name,`)
			assertContainsPattern(t, generatedStr, `func ConvertUserDTOToUser\(from \*UserDTO\) \*User`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `Name:\s+from.UserName,`)
		},
	},
	{
		name:          "package_level_conversion",
		directivePath: "../../testdata/02_basic_conversions/package_level_conversion",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/source",
			"github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/target",
		},
		priority: "P0",
		category: "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertUserSourceToUserTarget\(from \*UserSource\) \*UserTarget`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
			assertContainsPattern(t, generatedStr, `func ConvertUserTargetToUserSource\(from \*UserTarget\) \*UserSource`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
			assertContainsPattern(t, generatedStr, `func ConvertItemSourceToItemTarget\(from \*ItemSource\) \*ItemTarget`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
			assertContainsPattern(t, generatedStr, `func ConvertItemTargetToItemSource\(from \*ItemTarget\) \*ItemSource`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
		},
	},
	{
		name:          "oneway_conversion",
		directivePath: "../../testdata/02_basic_conversions/oneway_conversion",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/02_basic_conversions/oneway_conversion/source",
			"github.com/origadmin/abgen/testdata/02_basic_conversions/oneway_conversion/target",
		},
		priority: "P0",
		category: "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
			assertNotContainsPattern(t, generatedStr, `func ConvertUserDTOToUser`)
		},
	},
	{
		name:          "id_to_id_field_conversion",
		directivePath: "../../testdata/02_basic_conversions/id_to_id_field_conversion",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/02_basic_conversions/id_to_id_field_conversion/source",
			"github.com/origadmin/abgen/testdata/02_basic_conversions/id_to_id_field_conversion/target",
		},
		priority: "P0",
		category: "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertUserSourceToUserTarget\(from \*UserSource\) \*UserTarget`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.Id,`)
			assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
			assertContainsPattern(t, generatedStr, `func ConvertUserTargetToUserSource\(from \*UserTarget\) \*UserSource`)
			assertContainsPattern(t, generatedStr, `Id:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
		},
	},
	{
		name:          "simple_bilateral",
		directivePath: "../../testdata/02_basic_conversions/simple_bilateral",
		dependencies:  baseDependencies,
		priority:      "P0",
		category:      "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertStringToTime\(s string\) time.Time`)
			assertContainsPattern(t, generatedStr, `func ConvertTimeToString\(t time.Time\) string`)
			assertContainsPattern(t, generatedStr, `func ConvertUserToUserBilateral\(from \*User\) \*UserBilateral`)
			assertContainsPattern(t, generatedStr, `Id:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `Username:\s+from.Username,`)
			assertContainsPattern(t, generatedStr, `Age:\s+from.Age,`)
			assertContainsPattern(t, generatedStr, `Gender:\s+ConvertGenderToGenderBilateral\(from.Gender\),`)
			assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserStatusToUserBilateralStatus\(from.Status\),`)
			assertContainsPattern(t, generatedStr, `CreatedAt:\s+ConvertTimeToString\(from.CreatedAt\),`)
			assertNotContainsPattern(t, generatedStr, `Password:`)
			assertNotContainsPattern(t, generatedStr, `Salt:`)
			assertNotContainsPattern(t, generatedStr, `RoleIDs:`)
			assertNotContainsPattern(t, generatedStr, `Roles:`)
			assertNotContainsPattern(t, generatedStr, `Edges:`)
			assertContainsPattern(t, generatedStr, `func ConvertUserBilateralToUser\(from \*UserBilateral\) \*User`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.Id,`)
			assertContainsPattern(t, generatedStr, `Username:\s+from.Username,`)
			assertContainsPattern(t, generatedStr, `Age:\s+from.Age,`)
			assertContainsPattern(t, generatedStr, `Gender:\s+ConvertGenderBilateralToGender\(from.Gender\),`)
			assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserBilateralStatusToUserStatus\(from.Status\),`)
			assertContainsPattern(t, generatedStr, `CreatedAt:\s+ConvertStringToTime\(from.CreatedAt\),`)
		},
	},
	{
		name:          "standard_trilateral",
		directivePath: "../../testdata/02_basic_conversions/standard_trilateral",
		dependencies:  baseDependencies,
		priority:      "P0",
		category:      "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertStringToTime\(s string\) time.Time`)
			assertContainsPattern(t, generatedStr, `func ConvertTimeToString\(t time.Time\) string`)
			assertContainsPattern(t, generatedStr, `func ConvertUserToUserTrilateral\(from \*User\) \*UserTrilateral`)
			assertContainsPattern(t, generatedStr, `Id:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `Username:\s+from.Username,`)
			assertContainsPattern(t, generatedStr, `Age:\s+from.Age,`)
			assertContainsPattern(t, generatedStr, `Gender:\s+ConvertGenderToGenderTrilateral\(from.Gender\),`)
			assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserStatusToUserTrilateralStatus\(from.Status\),`)
			assertContainsPattern(t, generatedStr, `CreatedAt:\s+ConvertTimeToString\(from.CreatedAt\),`)
			assertContainsPattern(t, generatedStr, `func ConvertUserTrilateralToUser\(from \*UserTrilateral\) \*User`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.Id,`)
			assertContainsPattern(t, generatedStr, `Username:\s+from.Username,`)
			assertContainsPattern(t, generatedStr, `Age:\s+from.Age,`)
			assertContainsPattern(t, generatedStr, `Gender:\s+ConvertGenderTrilateralToGender\(from.Gender\),`)
			assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserTrilateralStatusToUserStatus\(from.Status\),`)
			assertContainsPattern(t, generatedStr, `CreatedAt:\s+ConvertStringToTime\(from.CreatedAt\),`)
			assertContainsPattern(t, generatedStr, `func ConvertResourceToResourceTrilateral\(from \*Resource\) \*ResourceTrilateral`)
			assertContainsPattern(t, generatedStr, `func ConvertResourceTrilateralToResource\(from \*ResourceTrilateral\) \*Resource`)
		},
	},
	{
		name:           "field_ignore_remap",
		directivePath:  "../../testdata/02_basic_conversions/field_ignore_remap",
		goldenFileName: "expected.gen.go",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/02_basic_conversions/field_ignore_remap/source",
			"github.com/origadmin/abgen/testdata/02_basic_conversions/field_ignore_remap/target",
		},
		priority: "P0",
		category: "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
			assertNotContainsPattern(t, generatedStr, `Password:`)
			assertNotContainsPattern(t, generatedStr, `CreatedAt:`)
			assertContainsPattern(t, generatedStr, `FullName:\s+from.Name,`)
			assertContainsPattern(t, generatedStr, `UserEmail:\s+from.Email,`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `LastUpdate:\s+from.UpdatedAt,`)
			assertNotContainsPattern(t, generatedStr, `CreatedDate:`)
		},
	},
	{
		name:           "slice_conversion",
		directivePath:  "../../testdata/02_basic_conversions/slice_conversion",
		goldenFileName: "expected.gen.go",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/02_basic_conversions/slice_conversion/source",
			"github.com/origadmin/abgen/testdata/02_basic_conversions/slice_conversion/target",
		},
		priority: "P0",
		category: "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertContainerVVSourceToContainerVVTarget\(from \*ContainerVVSource\) \*ContainerVVTarget`)
			assertContainsPattern(t, generatedStr, `Users:\s+ConvertUserVVsSourceToUserVVsTarget\(from.Users\),`)
			assertContainsPattern(t, generatedStr, `func ConvertUserVVsSourceToUserVVsTarget\(froms UserVVsSource\) UserVVsTarget`)
			assertContainsPattern(t, generatedStr, `tos\[i\] = \*ConvertUserVVSourceToUserVVTarget\(&f\)`)
			assertContainsPattern(t, generatedStr, `func ConvertContainerPPSourceToContainerPPTarget\(from \*ContainerPPSource\) \*ContainerPPTarget`)
			assertContainsPattern(t, generatedStr, `Users:\s+ConvertUserPPsSourceToUserPPsTarget\(from.Users\),`)
			assertContainsPattern(t, generatedStr, `func ConvertUserPPsSourceToUserPPsTarget\(froms UserPPsSource\) UserPPsTarget`)
			assertContainsPattern(t, generatedStr, `tos\[i\] = ConvertUserPPSourceToUserPPTarget\(f\)`)
			assertContainsPattern(t, generatedStr, `func ConvertContainerPVSourceToContainerPVTarget\(from \*ContainerPVSource\) \*ContainerPVTarget`)
			assertContainsPattern(t, generatedStr, `Users:\s+ConvertUserVPsSourceToUserVPsTarget\(from.Users\),`)
			assertContainsPattern(t, generatedStr, `func ConvertUserVPsSourceToUserVPsTarget\(froms UserVPsSource\) UserVPsTarget`)
			assertContainsPattern(t, generatedStr, `for i, f := range from`)
			assertContainsPattern(t, generatedStr, `return tos`)
			assertContainsPattern(t, generatedStr, `func ConvertContainerPPPSourceToContainerPPPTarget\(from \*ContainerPPPSource\) \*ContainerPPPTarget`)
			assertContainsPattern(t, generatedStr, `Users:\s+ConvertUserPPPsSourceToUserPPPsTarget\(from.Users\),`)
			assertContainsPattern(t, generatedStr, `func ConvertUserPPPsSourceToUserPPPsTarget\(froms UsersPPPsSource\) UsersPPPsTarget`)
			assertContainsPattern(t, generatedStr, `for i, f := range from`)
			assertContainsPattern(t, generatedStr, `return tos`)
			assertContainsPattern(t, generatedStr, `func ConvertContainerVPSourceToContainerVPTarget\(from \*ContainerVPSource\) \*ContainerVPTarget`)
			assertContainsPattern(t, generatedStr, `Users:\s+ConvertUserVPsSourceToUserVPsTarget\(from.Users\),`)
			assertContainsPattern(t, generatedStr, `func ConvertUserVPsSourceToUserVPsTarget\(froms UserVPsSource\) UserVPsTarget`)
			assertContainsPattern(t, generatedStr, `tmpVal := \*ConvertUserVPSourceToUserVPTarget\(&f\)`)
			assertContainsPattern(t, generatedStr, `tos\[i\] = &tmpVal`)
			assertContainsPattern(t, generatedStr, `func ConvertContainerPV2SourceToContainerPV2Target\(from \*ContainerPV2Source\) \*ContainerPV2Target`)
			assertContainsPattern(t, generatedStr, `Users:\s+ConvertUserPV2sSourceToUserPV2sTarget\(from.Users\),`)
			assertContainsPattern(t, generatedStr, `func ConvertUserPV2sSourceToUserPV2sTarget\(froms UserPV2sSource\) UserPV2sTarget`)
			assertContainsPattern(t, generatedStr, `tos\[i\] = \*ConvertUserPV2SourceToUserPV2Target\(f\)`)
			assertContainsPattern(t, generatedStr, `func ConvertOrderSourceToOrderTarget\(from \*OrderSource\) \*OrderTarget`)
			assertContainsPattern(t, generatedStr, `Items:\s+ConvertItemsSourceToItemsTarget\(from.Items\),`)
		},
	},
	// === 03_advanced_features: Advanced Feature Tests ===
	{
		name:           "auto_generate_aliases",
		directivePath:  "../../testdata/03_advanced_features/auto_generate_aliases",
		goldenFileName: "expected.gen.go",
		dependencies:   baseDependencies,
		priority:       "P0",
		category:       "advanced_features",
	},
	{
		name:          "custom_function_rules",
		directivePath: "../../testdata/03_advanced_features/custom_function_rules",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/source",
			"github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/target",
		},
		priority: "P0",
		category: "advanced_features",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			stubStr := string(stubCode)
			slog.Debug("Generated code inside custom_function_rules assertFunc", "code", generatedStr)
			assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserStatusToUserCustomStatus\(from.Status\),`)
			assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserCustomStatusToUserStatus\(from.Status\),`)
			assertContainsPattern(t, stubStr, `func ConvertUserStatusToUserCustomStatus\(from int\) string`)
		},
	},
	{
		name:           "slice_conversions",
		directivePath:  "../../testdata/03_advanced_features/slice_conversions",
		goldenFileName: "expected.gen.go",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversions/source",
			"github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversions/target",
		},

		priority: "P0",
		category: "advanced_features",
	},
	{
		name:          "enum_string_to_int",
		directivePath: "../../testdata/03_advanced_features/enum_string_to_int",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/03_advanced_features/enum_string_to_int/source",
			"github.com/origadmin/abgen/testdata/03_advanced_features/enum_string_to_int/target",
		}, priority: "P1",
		category:    "advanced_features",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			t.Log("TODO: Add specific assertions for enum_string_to_int")
		},
	},
	{
		name:          "pointer_conversions",
		directivePath: "../../testdata/03_advanced_features/pointer_conversions",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/03_advanced_features/pointer_conversions/source",
			"github.com/origadmin/abgen/testdata/03_advanced_features/pointer_conversions/target",
		},

		priority: "P1",
		category: "advanced_features",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			t.Log("TODO: Add specific assertions for pointer_conversions")
		},
	},
	{
		name:          "map_conversions",
		directivePath: "../../testdata/03_advanced_features/map_conversions",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/03_advanced_features/map_conversions/source",
			"github.com/origadmin/abgen/testdata/03_advanced_features/map_conversions/target",
		},
		priority: "P1",
		category: "advanced_features",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			t.Log("TODO: Add specific assertions for map_conversions")
		},
	},
	{
		name:          "numeric_conversions",
		directivePath: "../../testdata/03_advanced_features/numeric_conversions",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/03_advanced_conversions/numeric_conversions/source",
			"github.com/origadmin/abgen/testdata/03_advanced_conversions/numeric_conversions/target",
		},
		priority: "P1",
		category: "advanced_features",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			t.Log("TODO: Add specific assertions for numeric_conversions")
		},
	},
	// === 06_regression: Regression Tests ===
	{
		name:           "array_slice_test",
		directivePath:  "../../testdata/06_regression/array_slice_fix/array_slice_test",
		goldenFileName: "expected.gen.go",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/source",
			"github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/target",
		},
		priority: "P0",
		category: "regression",
	},
	{
		name:           "alias-gen",
		directivePath:  "../../testdata/06_regression/alias_gen_fix",
		goldenFileName: "expected.gen.go",
		dependencies:   baseDependencies,
		priority:       "P0",
		category:       "regression",
	},
	{
		name:          "map_string_to_string_conversion",
		directivePath: "../../testdata/06_regression/map_string_to_string_fix",
		dependencies: []string{
			"github.com/origadmin/abgen/testdata/06_regression/map_string_to_string_fix/source",
			"github.com/origadmin/abgen/testdata/06_regression/map_string_to_string_fix/target",
		},
		priority: "P0",
		category: "regression",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			stubStr := string(stubCode)
			t.Logf("Generated code for map_string_to_string_conversion:\n%s", generatedStr)
			if len(stubCode) > 0 {
				t.Logf("Generated stub code:\n%s", stubCode)
			}
			assertContainsPattern(t, generatedStr, `func ConvertMapToStringSourceToMapToStringTarget\(from \*MapToStringSource\) \*MapToStringTarget`)
			assertContainsPattern(t, generatedStr, `func ConvertMapToStringTargetToMapToStringSource\(from \*MapToStringTarget\) \*MapToStringSource`)
			if len(stubStr) > 0 {
				assertContainsPattern(t, stubStr, `func ConvertMapToStringSourceMetadataToMapToStringTargetMetadata`)
				assertContainsPattern(t, stubStr, `func ConvertMapToStringSourceTagsToMapToStringTargetTags`)
				assertContainsPattern(t, stubStr, `func ConvertMapToStringSourceConfigToMapToStringTargetConfig`)
				assertContainsPattern(t, stubStr, `func ConvertMapToStringTargetMetadataToMapToStringSourceMetadata`)
				assertContainsPattern(t, stubStr, `func ConvertMapToStringTargetTagsToMapToStringSourceTags`)
				assertContainsPattern(t, stubStr, `func ConvertMapToStringTargetConfigToMapToStringSourceConfig`)
				t.Logf("Stub functions generated with correct naming pattern for map->string conversion")
			} else {
				assertContainsPattern(t, generatedStr, `ConvertMapToStringSourceMetadataToMapToStringTargetMetadata`)
				t.Logf("Main code handles map->string conversion through named conversion functions")
			}
		},
	},
}

func TestLegacyGenerator_Generate(t *testing.T) {
	t.Skip()

	sort.Slice(testCases, func(i, j int) bool {
		priorityOrder := map[string]int{"P0": 0, "P1": 1, "P2": 2}
		if priorityOrder[testCases[i].priority] != priorityOrder[testCases[j].priority] {
			return priorityOrder[testCases[i].priority] < priorityOrder[testCases[j].priority]
		}
		return testCases[i].category < testCases[j].category
	})

	for _, tc := range testCases {
		var stagePrefix string
		pathParts := strings.Split(tc.directivePath, "/")
		for _, part := range pathParts {
			if len(part) > 2 && part[2] == '_' && part[0] >= '0' && part[0] <= '9' && part[1] >= '0' && part[1] <= '9' {
				stagePrefix = part[:2]
				break
			}
		}

		testNameWithStage := fmt.Sprintf("%s_%s/%s", stagePrefix, tc.category, tc.name)
		t.Run(testNameWithStage, func(t *testing.T) {
			t.Logf("Running test: %s (Priority: %s, Category: %s)", tc.name, tc.priority, tc.category)
			cleanTestFiles(t, tc.directivePath)

			parser := config.NewParser()
			cfg, err := parser.Parse(tc.directivePath)
			if err != nil {
				t.Fatalf("config.Parser.Parse() failed: %v", err)
			}
			if tc.name == "field_ignore_remap" {
				t.Logf("Parsed config for field_ignore_remap: %+v", cfg.ConversionRules)
			}

			typeAnalyzer := analyzer.NewTypeAnalyzer()
			typeInfos, err := typeAnalyzer.Analyze(cfg)
			if err != nil {
				t.Fatalf("analyzer.TypeAnalyzer.Analyze() failed: %v", err)
			}

			g := NewCodeGenerator()
			response, err := g.Generate(cfg, typeInfos)
			if err != nil {
				t.Fatalf("Generate() failed for test case %s: %v", tc.name, err)
			}
			generatedCode, stubCode := response.GeneratedCode, response.CustomStubs

			generatedCodeStr := string(generatedCode)
			generatedCodeStr = strings.ReplaceAll(generatedCodeStr, `\`, `/`)
			generatedCode = []byte(generatedCodeStr)

			if tc.assertFunc != nil {
				t.Run(tc.name+"_Assertions", func(st *testing.T) {
					tc.assertFunc(st, generatedCode, stubCode)
					actualOutputFile := filepath.Join(tc.directivePath, "expected.actual.gen.go")
					_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
				})
			}

			if tc.goldenFileName != "" {
				goldenFile := filepath.Join(tc.directivePath, tc.goldenFileName)
				if os.Getenv("UPDATE_GOLDEN_FILES") != "" {
					err = os.WriteFile(goldenFile, generatedCode, 0644)
					if err != nil {
						t.Fatalf("Failed to update golden file %s: %v", goldenFile, err)
					}
					t.Logf("Updated golden file: %s", goldenFile)
					return
				}

				expectedCode, err := os.ReadFile(goldenFile)
				if err != nil {
					t.Fatalf("Failed to read golden file %s: %v", goldenFile, err)
				}

				if string(generatedCode) != string(expectedCode) {
					actualOutputFile := filepath.Join(tc.directivePath, "failed.actual.gen.go")
					_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
					t.Errorf("Generated code for '%s' does not match the golden file %s. The generated output was saved to %s for inspection.", tc.name, goldenFile, actualOutputFile)
				}
			} else {
				t.Logf("Skipping golden file comparison for test: %s (no goldenFileName provided)", tc.name)
			}
		})
	}
}

func TestDefaultDirectionBehavior(t *testing.T) {
	slog.Debug("Starting TestDefaultDirectionBehavior")
	testPath := "../../testdata/02_basic_conversions/simple_struct"
	slog.Debug("Testing directory", "path", testPath)
	parser := config.NewParser()
	config, err := parser.Parse(testPath)
	if err != nil {
		slog.Error("Failed to parse configuration", "error", err)
		t.Fatalf("Failed to parse configuration: %v", err)
	}
	slog.Debug("Parsed configuration", "rules_count", len(config.ConversionRules))
	if len(config.ConversionRules) == 0 {
		t.Fatal("Expected at least one conversion rule, got none")
	}
	rule := config.ConversionRules[0]
	slog.Debug("First rule", "source", rule.SourceType, "target", rule.TargetType, "direction", rule.Direction)
	if rule.Direction != "both" {
		slog.Error("Direction should default to 'both'", "actual", rule.Direction)
		t.Errorf("Expected direction to be 'both', got '%s'", rule.Direction)
	}
	slog.Debug("Direction assertion passed")
	slog.Debug("TestDefaultDirectionBehavior completed successfully")
}

func cleanTestFiles(t *testing.T, dir string) {
	files, err := filepath.Glob(filepath.Join(dir, "*.gen.go"))
	if err != nil {
		t.Fatalf("Failed to glob for generated files in %s: %v", dir, err)
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			t.Logf("Warning: Failed to remove old generated file %s: %v", f, err)
		}
	}
}

func assertContainsPattern(t *testing.T, code string, pattern string) {
	t.Helper()
	match, err := regexp.MatchString(pattern, code)
	if err != nil {
		t.Fatalf("Invalid regex pattern %q: %v", pattern, err)
	}
	if !match {
		t.Errorf("Generated code does not contain expected pattern %q.\nGenerated Code:\n%s", pattern, code)
	}
}

func assertNotContainsPattern(t *testing.T, code string, pattern string) {
	t.Helper()
	match, err := regexp.MatchString(pattern, code)
	if err != nil {
		t.Fatalf("Invalid regex pattern %q: %v", pattern, err)
	}
	if match {
		t.Errorf("Generated code contains unexpected pattern %q.\nGenerated Code:\n%s", pattern, code)
	}
}

func TestOrchestratorBasicFunctionality(t *testing.T) {
	t.Skip()

	testPath := "../../testdata/02_basic_conversions/simple_struct"
	parser := config.NewParser()
	cfg, err := parser.Parse(testPath)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}
	typeAnalyzer := analyzer.NewTypeAnalyzer()
	typeInfos, err := typeAnalyzer.Analyze(cfg)
	if err != nil {
		t.Fatalf("Failed to analyze types: %v", err)
	}

	t.Run("Create_CodeGenerator", func(t *testing.T) {
		orchestrator := NewCodeGenerator()
		if orchestrator == nil {
			t.Fatal("Failed to create orchestrator")
		}
	})

	t.Run("Generate_Code", func(t *testing.T) {
		orchestrator := NewCodeGenerator()
		response, err := orchestrator.Generate(cfg, typeInfos)
		if err != nil {
			t.Fatalf("Generation failed: %v", err)
		}
		if len(response.GeneratedCode) == 0 {
			t.Fatal("GeneratedCode is empty")
		}
		generatedStr := string(response.GeneratedCode)
		assertContainsPattern(t, generatedStr, `func Convert.*To.*\(from \*.*\) \*.*`)
		assertContainsPattern(t, generatedStr, `ID:\s+from\.ID,`)
		assertContainsPattern(t, generatedStr, `Name:\s+from\.Name,`)
		t.Logf("Successfully generated %d bytes with %d packages",
			len(response.GeneratedCode), len(response.RequiredPackages))
	})
}

func TestCodeGenerator_Generate(t *testing.T) {
	//baseDependencies := []string{
	//	"github.com/origadmin/abgen/testdata/fixtures/ent",
	//	"github.com/origadmin/abgen/testdata/fixtures/types",
	//}
	//testCases := []struct {
	//	name           string
	//	directivePath  string
	//	goldenFileName string
	//	dependencies   []string
	//	priority       string
	//	category       string
	//	assertFunc     func(t *testing.T, generatedCode []byte, stubCode []byte)
	//}{
	//	{
	//		name:          "simple_struct_conversion",
	//		directivePath: "../../testdata/02_basic_conversions/simple_struct",
	//		dependencies: []string{
	//			"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/source",
	//			"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/target",
	//		},
	//		priority: "P0",
	//		category: "basic_conversions",
	//		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
	//			generatedStr := string(generatedCode)
	//			assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
	//			assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
	//			assertContainsPattern(t, generatedStr, `UserName:\s+from.Name,`)
	//			assertContainsPattern(t, generatedStr, `func ConvertUserDTOToUser\(from \*UserDTO\) \*User`)
	//			assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
	//			assertContainsPattern(t, generatedStr, `Name:\s+from.UserName,`)
	//		},
	//	},
	//	{
	//		name:          "package_level_conversion",
	//		directivePath: "../../testdata/02_basic_conversions/package_level_conversion",
	//		dependencies: []string{
	//			"github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/source",
	//			"github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/target",
	//		},
	//		priority: "P0",
	//		category: "basic_conversions",
	//		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
	//			generatedStr := string(generatedCode)
	//			assertContainsPattern(t, generatedStr, `func ConvertUserSourceToUserTarget\(from \*UserSource\) \*UserTarget`)
	//			assertContainsPattern(t, generatedStr, `func ConvertUserTargetToUserSource\(from \*UserTarget\) \*UserSource`)
	//			assertContainsPattern(t, generatedStr, `func ConvertItemSourceToItemTarget\(from \*ItemSource\) \*ItemTarget`)
	//			assertContainsPattern(t, generatedStr, `func ConvertItemTargetToItemSource\(from \*ItemTarget\) \*ItemSource`)
	//		},
	//	},
	//	{
	//		name:          "oneway_conversion",
	//		directivePath: "../../testdata/02_basic_conversions/oneway_conversion",
	//		dependencies: []string{
	//			"github.com/origadmin/abgen/testdata/02_basic_conversions/oneway_conversion/source",
	//			"github.com/origadmin/abgen/testdata/02_basic_conversions/oneway_conversion/target",
	//		},
	//		priority: "P0",
	//		category: "basic_conversions",
	//		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
	//			generatedStr := string(generatedCode)
	//			assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
	//			assertNotContainsPattern(t, generatedStr, `func ConvertUserDTOToUser`)
	//		},
	//	},
	//	{
	//		name:          "simple_bilateral",
	//		directivePath: "../../testdata/02_basic_conversions/simple_bilateral",
	//		dependencies:  baseDependencies,
	//		priority:      "P0",
	//		category:      "basic_conversions",
	//		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
	//			generatedStr := string(generatedCode)
	//			assertContainsPattern(t, generatedStr, `func ConvertStringToTime\(s string\) time.Time`)
	//			assertContainsPattern(t, generatedStr, `func ConvertTimeToString\(t time.Time\) string`)
	//			assertContainsPattern(t, generatedStr, `func ConvertUserToUserBilateral\(from \*User\) \*UserBilateral`)
	//			assertContainsPattern(t, generatedStr, `func ConvertUserBilateralToUser\(from \*UserBilateral\) \*User`)
	//			assertNotContainsPattern(t, generatedStr, `Password:`)
	//			assertNotContainsPattern(t, generatedStr, `Salt:`)
	//		},
	//	},
	//	{
	//		name:           "custom_function_rules",
	//		directivePath:  "../../testdata/03_advanced_features/custom_function_rules",
	//		goldenFileName: "expected.gen.go",
	//		dependencies: []string{
	//			"github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/source",
	//			"github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/target",
	//		},
	//		priority: "P0",
	//		category: "advanced_features",
	//		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
	//			generatedStr := string(generatedCode)
	//			stubStr := string(stubCode)
	//			assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserStatusToUserCustomStatus\(from.Status\),`)
	//			assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserCustomStatusToUserStatus\(from.Status\),`)
	//			if len(stubStr) > 0 {
	//				assertContainsPattern(t, stubStr, `func ConvertUserStatusToUserCustomStatus\(from int\) string`)
	//			}
	//		},
	//	},
	//}

	sort.Slice(testCases, func(i, j int) bool {
		priorityOrder := map[string]int{"P0": 0, "P1": 1, "P2": 2}
		if priorityOrder[testCases[i].priority] != priorityOrder[testCases[j].priority] {
			return priorityOrder[testCases[i].priority] < priorityOrder[testCases[j].priority]
		}
		return testCases[i].category < testCases[j].category
	})

	for _, tc := range testCases {
		var stagePrefix string
		pathParts := strings.Split(tc.directivePath, "/")
		for _, part := range pathParts {
			if len(part) > 2 && part[2] == '_' && part[0] >= '0' && part[0] <= '9' && part[1] >= '0' && part[1] <= '9' {
				stagePrefix = part[:2]
				break
			}
		}

		testNameWithStage := fmt.Sprintf("%s_%s/%s", stagePrefix, tc.category, tc.name)
		t.Run(testNameWithStage, func(t *testing.T) {
			t.Logf("Running test: %s (Priority: %s, Category: %s)", tc.name, tc.priority, tc.category)
			cleanTestFiles(t, tc.directivePath)

			parser := config.NewParser()
			cfg, err := parser.Parse(tc.directivePath)
			if err != nil {
				t.Fatalf("config.Parser.Parse() failed: %v", err)
			}

			typeAnalyzer := analyzer.NewTypeAnalyzer()
			typeInfos, err := typeAnalyzer.Analyze(cfg)
			if err != nil {
				t.Fatalf("analyzer.TypeAnalyzer.Analyze() failed: %v", err)
			}

			orchestrator := NewCodeGenerator()
			response, err := orchestrator.Generate(cfg, typeInfos)
			if err != nil {
				t.Fatalf("Generate() failed for test case %s: %v", tc.name, err)
			}

			generatedCode := response.GeneratedCode
			stubCode := response.CustomStubs

			generatedCodeStr := string(generatedCode)
			generatedCodeStr = strings.ReplaceAll(generatedCodeStr, `\`, `/`)
			generatedCode = []byte(generatedCodeStr)

			// Always save the actual generated code for inspection, regardless of assertFunc
			actualOutputFile := filepath.Join(tc.directivePath, "actual.gen.go")
			actualOutputFile, _ = filepath.Abs(actualOutputFile)
			t.Logf("Attempting to save generated code to: %s", actualOutputFile)
			if err := os.WriteFile(actualOutputFile, generatedCode, 0644); err != nil {
				t.Logf("Failed to save actual output to %s: %v", actualOutputFile, err)
				// Try to get more detailed error information
				if _, statErr := os.Stat(tc.directivePath); statErr != nil {
					t.Logf("Directory %s does not exist or is not accessible: %v", tc.directivePath, statErr)
				}
			} else {
				// Verify the file was actually created
				if _, err := os.Stat(actualOutputFile); err == nil {
					t.Logf("Generated output successfully saved to %s for inspection", actualOutputFile)
					t.Logf("File size: %d bytes", len(generatedCode))
				} else {
					t.Logf("File save reported success but file not found at %s: %v", actualOutputFile, err)
				}
			}

			if len(stubCode) > 0 {
				actualStubFile := filepath.Join(tc.directivePath, "actual.stub.go")
				t.Logf("Attempting to save stub code to: %s", actualStubFile)
				if err := os.WriteFile(actualStubFile, stubCode, 0644); err != nil {
					t.Logf("Failed to save stub output to %s: %v", actualStubFile, err)
				} else {
					if _, err := os.Stat(actualStubFile); err == nil {
						t.Logf("Stub output saved to %s for inspection", actualStubFile)
						t.Logf("Stub file size: %d bytes", len(stubCode))
					} else {
						t.Logf("Stub file save reported success but file not found at %s: %v", actualStubFile, err)
					}
				}
			}

			if tc.assertFunc != nil {
				tc.assertFunc(t, generatedCode, stubCode)

				if t.Failed() {
					t.Logf("Assertion failed for '%s'. Generated output is available at %s", tc.name,
						actualOutputFile)
				}
			}

			if tc.goldenFileName != "" {
				goldenFile := filepath.Join(tc.directivePath, tc.goldenFileName)
				if os.Getenv("UPDATE_GOLDEN_FILES") != "" {
					err = os.WriteFile(goldenFile, generatedCode, 0644)
					if err != nil {
						t.Fatalf("Failed to update golden file %s: %v", goldenFile, err)
					}
					t.Logf("Updated golden file: %s", goldenFile)
					return
				}
			}
		})
	}
}

func TestArchitecturalCompatibility(t *testing.T) {
	t.Skip()

	if testing.Short() {
		t.Skip("Skipping compatibility test in short mode")
	}

	testCases := []struct {
		name          string
		directivePath string
		dependencies  []string
	}{
		{
			name:          "simple_struct",
			directivePath: "../../testdata/02_basic_conversions/simple_struct",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/target",
			},
		},
		{
			name:          "simple_bilateral",
			directivePath: "../../testdata/02_basic_conversions/simple_bilateral",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/fixtures/ent",
				"github.com/origadmin/abgen/testdata/fixtures/types",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing architectural compatibility for %s", tc.name)

			parser := config.NewParser()
			cfg, err := parser.Parse(tc.directivePath)
			if err != nil {
				t.Fatalf("Failed to parse config: %v", err)
			}

			typeAnalyzer := analyzer.NewTypeAnalyzer()
			typeInfos, err := typeAnalyzer.Analyze(cfg)
			if err != nil {
				t.Fatalf("Failed to analyze types: %v", err)
			}

			orchestrator := NewCodeGenerator()
			response, err := orchestrator.Generate(cfg, typeInfos)
			if err != nil {
				t.Fatalf("NEW architecture failed: %v", err)
			}
			newCode := response.GeneratedCode

			newStr := strings.ReplaceAll(string(newCode), `\`, `/`)

			t.Errorf("Architecture mismatch for %s", tc.name)
			t.Logf("NEW architecture output length: %d", len(newStr))

			outputDir := filepath.Join(tc.directivePath, "compatibility_test")
			_ = os.MkdirAll(outputDir, 0755)
			_ = os.WriteFile(filepath.Join(outputDir, "new_architecture.gen.go"), []byte(newStr), 0644)
			t.Logf("Output files written to %s for manual comparison", outputDir)
		})
	}
}

func TestNewArchitectureComponents(t *testing.T) {
	t.Skip()

	if testing.Short() {
		t.Skip("Skipping component tests in short mode")
	}

	testPath := "../../testdata/02_basic_conversions/simple_struct"
	parser := config.NewParser()
	cfg, err := parser.Parse(testPath)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	typeAnalyzer := analyzer.NewTypeAnalyzer()
	typeInfos, err := typeAnalyzer.Analyze(cfg)
	if err != nil {
		t.Fatalf("Failed to analyze types: %v", err)
	}

	t.Run("CodeGenerator_Creation", func(t *testing.T) {
		orchestrator := NewCodeGenerator()
		if orchestrator == nil {
			t.Fatal("Failed to create orchestrator")
		}
	})

	t.Run("End_to_End_Generation", func(t *testing.T) {
		orchestrator := NewCodeGenerator()
		gen, ok := orchestrator.(*CodeGenerator)
		if !ok {
			t.Fatalf("Orchestrator is not a CodeGenerator")
		}
		response, err := gen.Generate(cfg, typeInfos)
		if err != nil {
			t.Fatalf("Generation failed: %v", err)
		}
		if len(response.GeneratedCode) == 0 {
			t.Fatal("GeneratedCode is empty in response")
		}
		generatedStr := string(response.GeneratedCode)
		assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
		assertContainsPattern(t, generatedStr, `func ConvertUserDTOToUser\(from \*UserDTO\) \*User`)
		t.Logf("Successfully generated %d bytes of code with %d required packages",
			len(response.GeneratedCode), len(response.RequiredPackages))
	})
}
