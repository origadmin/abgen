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
)

func init() {
	slog.SetDefault(slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	))
}

var testCases = []struct {
	name           string
	directivePath  string
	goldenFileName string
	priority       string
	category       string
	assertFunc     func(t *testing.T, generatedCode []byte, stubCode []byte)
}{
	{
		name:          "simple_struct_conversion",
		directivePath: "../../testdata/02_basic_conversions/simple_struct",
		priority:      "P0",
		category:      "basic_conversions",
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
		priority:      "P0",
		category:      "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertUserSourceToUserTarget\(from \*UserSource\) \*UserTarget`)
			assertContainsPattern(t, generatedStr, `func ConvertUserTargetToUserSource\(from \*UserTarget\) \*UserSource`)
			assertContainsPattern(t, generatedStr, `func ConvertItemSourceToItemTarget\(from \*ItemSource\) \*ItemTarget`)
			assertContainsPattern(t, generatedStr, `func ConvertItemTargetToItemSource\(from \*ItemTarget\) \*ItemSource`)
		},
	},
	{
		name:          "oneway_conversion",
		directivePath: "../../testdata/02_basic_conversions/oneway_conversion",
		priority:      "P0",
		category:      "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
			assertNotContainsPattern(t, generatedStr, `func ConvertUserDTOToUser`)
		},
	},
	{
		name:          "id_to_id_field_conversion",
		directivePath: "../../testdata/02_basic_conversions/id_to_id_field_conversion",
		priority:      "P0",
		category:      "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `ID:\s+from.Id,`)
			assertContainsPattern(t, generatedStr, `Id:\s+from.ID,`)
		},
	},
	{
		name:          "simple_bilateral",
		directivePath: "../../testdata/02_basic_conversions/simple_bilateral",
		priority:      "P0",
		category:      "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertUserToUserBilateral\(from \*User\) \*UserBilateral`)
			assertContainsPattern(t, generatedStr, `Id:\s+from.ID,`)
			assertContainsPattern(t, generatedStr, `func ConvertUserBilateralToUser\(from \*UserBilateral\) \*User`)
			assertContainsPattern(t, generatedStr, `ID:\s+from.Id,`)
		},
	},
	{
		name:           "field_ignore_remap",
		directivePath:  "../../testdata/02_basic_conversions/field_ignore_remap",
		goldenFileName: "expected.gen.go",
		priority:       "P0",
		category:       "basic_conversions",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
			assertNotContainsPattern(t, generatedStr, `Password:`)
			assertNotContainsPattern(t, generatedStr, `CreatedAt:`)
			assertContainsPattern(t, generatedStr, `FullName:\s+from.Name,`)
			assertContainsPattern(t, generatedStr, `UserEmail:\s+from.Email,`)
		},
	},
	{
		name:           "slice_conversion",
		directivePath:  "../../testdata/02_basic_conversions/slice_conversion",
		goldenFileName: "expected.gen.go",
		priority:       "P0",
		category:       "basic_conversions",
	},
	{
		name:           "auto_generate_aliases",
		directivePath:  "../../testdata/03_advanced_features/auto_generate_aliases",
		goldenFileName: "expected.gen.go",
		priority:       "P0",
		category:       "advanced_features",
	},
	{
		name:          "custom_function_rules",
		directivePath: "../../testdata/03_advanced_features/custom_function_rules",
		priority:      "P0",
		category:      "advanced_features",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			stubStr := string(stubCode)
			assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserStatusToUserCustomStatus\(from.Status\),`)
			assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserCustomStatusToUserStatus\(from.Status\),`)
			assertContainsPattern(t, stubStr, `func ConvertUserStatusToUserCustomStatus\(from int\) string`)
		},
	},
	{
		name:           "array_slice_test",
		directivePath:  "../../testdata/06_regression/array_slice_test",
		goldenFileName: "expected.gen.go",
		priority:       "P0",
		category:       "regression",
	},
	{
		name:           "alias-gen",
		directivePath:  "../../testdata/06_regression/alias_gen_fix",
		goldenFileName: "expected.gen.go",
		priority:       "P0",
		category:       "regression",
	},
	{
		name:          "map_string_to_string_conversion",
		directivePath: "../../testdata/06_regression/map_string_to_string_fix",
		priority:      "P0",
		category:      "regression",
	},
	{
		name:          "menu_pb_fix",
		directivePath: "../../testdata/06_regression/menu_pb_fix",
		priority:      "P0",
		category:      "regression",
		assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
			generatedStr := string(generatedCode)
			assertContainsPattern(t, generatedStr, `Menu   = ent.Menu`)
			assertContainsPattern(t, generatedStr, `MenuPB = pb.Menu`)
			assertContainsPattern(t, generatedStr, `func ConvertMenuToMenuPB\(from \*Menu\) \*MenuPB`)
			assertContainsPattern(t, generatedStr, `func ConvertMenuPBToMenu\(from \*MenuPB\) \*Menu`)
		},
	},
}

func TestCodeGenerator_Generate(t *testing.T) {
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

			typeAnalyzer := analyzer.NewTypeAnalyzer()
			analysisResult, err := typeAnalyzer.Analyze(tc.directivePath)
			if err != nil {
				t.Logf("analyzer.TypeAnalyzer.Analyze() returned an error (may be expected for tests with dummy packages): %v", err)
				if analysisResult == nil {
					t.Fatalf("analyzer.TypeAnalyzer.Analyze() failed and returned a nil result: %v", err)
				}
			}

			response, err := Generate(analysisResult)
			if err != nil {
				t.Fatalf("Generate() failed for test case %s: %v", tc.name, err)
			}

			generatedCode := response.GeneratedCode
			stubCode := response.CustomStubs
			generatedCodeStr := strings.ReplaceAll(string(generatedCode), `\`, `/`)
			generatedCode = []byte(generatedCodeStr)

			actualOutputFile := filepath.Join(tc.directivePath, "actual.gen.go")
			if err := os.WriteFile(actualOutputFile, generatedCode, 0644); err != nil {
				t.Logf("Failed to save actual output to %s: %v", actualOutputFile, err)
			} else {
				t.Logf("Generated output successfully saved to %s for inspection", actualOutputFile)
			}

			if len(stubCode) > 0 {
				actualStubFile := filepath.Join(tc.directivePath, "actual.stub.go")
				if err := os.WriteFile(actualStubFile, stubCode, 0644); err != nil {
					t.Logf("Failed to save stub output to %s: %v", actualStubFile, err)
				} else {
					t.Logf("Stub output saved to %s for inspection", actualStubFile)
				}
			}

			if tc.assertFunc != nil {
				tc.assertFunc(t, generatedCode, stubCode)
				if t.Failed() {
					t.Logf("Assertion failed for '%s'. Generated output is available at %s", tc.name, actualOutputFile)
				}
			}

			if tc.goldenFileName != "" {
				goldenFile := filepath.Join(tc.directivePath, tc.goldenFileName)
				if os.Getenv("UPDATE_GOLDEN_FILES") != "" {
					if err := os.WriteFile(goldenFile, generatedCode, 0644); err != nil {
						t.Fatalf("Failed to update golden file %s: %v", goldenFile, err)
					}
					t.Logf("Updated golden file: %s", goldenFile)
					return
				}
			}
		})
	}
}

func cleanTestFiles(t *testing.T, dir string) {
	files, err := filepath.Glob(filepath.Join(dir, "*.gen.go"))
	if err != nil {
		t.Fatalf("Failed to glob for generated files in %s: %v", dir, err)
	}
	files = append(files, filepath.Join(dir, "actual.stub.go"))
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
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
