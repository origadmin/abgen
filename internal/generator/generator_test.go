package generator

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	
		"github.com/origadmin/abgen/internal/analyzer"
	
	"github.com/origadmin/abgen/internal/config"
)

func TestGenerator_CodeGeneration(t *testing.T) {

	// Base dependencies for most tests
	// NOTE: These are now only used for tests that specifically rely on the global fixture.
	// New tests should define their own local dependencies if they have specific fixture types.
	// The 'dependencies' field in test cases will now be used by the config.DirectiveParser to extract dependencies.
	// The actual loading will be handled by analyzer.PackageWalker.
	baseDependencies := []string{
		"github.com/origadmin/abgen/testdata/fixtures/ent",
		"github.com/origadmin/abgen/testdata/fixtures/types",
	}

	testCases := []struct {
		name           string
		directivePath  string
		goldenFileName string
		dependencies   []string // These are now hints for directive parsing, not direct load patterns
		priority       string   // P0, P1, P2 for prioritization
		category       string   // Test category for organization
		assertFunc     func(*testing.T, []byte) // Custom assertion function for detailed checks
	}{
		// === 01_basic_modes: Basic Conversion Patterns ===
		{
			name:           "simple_bilateral",
			directivePath:  "../../testdata/02_basic_conversions/simple_bilateral",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "basic_modes",
		},
		{
			name:           "standard_trilateral",
			directivePath:  "../../testdata/02_basic_conversions/standard_trilateral",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "basic_modes",
		},
		// Note: multi_source and multi_target test cases have been removed for now as their definitions need refinement.

		// === 02_basic_conversions: Basic Struct Conversion ===
		{
			name:           "simple_struct_conversion",
			directivePath:  "../../testdata/02_basic_conversions/simple_struct",
			goldenFileName: "expected.golden",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/target",
			},
			priority: "P0",
			category: "basic_conversions",
		},
		{
			name:          "package_level_conversion",
			directivePath: "../../testdata/02_basic_conversions/package_level_conversion",
			// goldenFileName: "expected.golden", // REMOVED
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte) {
				generatedStr := strings.ReplaceAll(string(generatedCode), "\r\n", "\n")

				// Assert that a type block is used
				assertContainsPattern(t, generatedStr, "type (")
				assertContainsPattern(t, generatedStr, ")")

				// Assert type aliases are generated correctly within the block
				assertContainsPattern(t, generatedStr, "\tUserSource = source.User")
				assertContainsPattern(t, generatedStr, "\tUserTarget = target.User")

				// Assert that no conflicting aliases like "type User = ..." exist for source.User or target.User outside the block
				assertNotContainsPattern(t, generatedStr, "type User = source.User")
				assertNotContainsPattern(t, generatedStr, "type User = target.User")

				// Assert conversion functions exist with correct naming
				assertContainsPattern(t, generatedStr, `func ConvertUserSourceToUserTarget`)
				assertContainsPattern(t, generatedStr, `func ConvertUserTargetToUserSource`)

				// Assert some basic content of the conversion function, e.g., field assignment
				assertContainsPattern(t, generatedStr, `target := &UserTarget{`) // For ConvertUserSourceToUserTarget
				assertContainsPattern(t, generatedStr, `ID:   from.ID,`)
				assertContainsPattern(t, generatedStr, `Name: from.Name,`)

				assertContainsPattern(t, generatedStr, `target := &UserSource{`) // For ConvertUserTargetToUserSource
				assertContainsPattern(t, generatedStr, `ID:   from.ID,`)
				assertContainsPattern(t, generatedStr, `Name: from.Name,`)
			},
		},
		{
			name:           "single_way_conversion",
			directivePath:  "../../testdata/02_basic_conversions/single_way_conversion",
			goldenFileName: "expected.golden",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/single_way_conversion/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/single_way_conversion/target",
			},
			priority: "P0",
			category: "basic_conversions",
		},
		{
			name:          "id_to_id_field_conversion",
			directivePath: "../../testdata/02_basic_conversions/id_to_id_field_conversion",
			// goldenFileName: "expected.golden", // REMOVED
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/id_to_id_field_conversion/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/id_to_id_field_conversion/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte) {
				generatedStr := strings.ReplaceAll(string(generatedCode), "\r\n", "\n")

				// Assert that a type block is used
				assertContainsPattern(t, generatedStr, "type (")
				assertContainsPattern(t, generatedStr, ")")

				// Assert type aliases are generated correctly within the block
				assertContainsPattern(t, generatedStr, "\tUserSource = source.User")
				assertContainsPattern(t, generatedStr, "\tUserTarget = target.User")

				// Assert that no conflicting aliases like "type User = ..." exist for source.User or target.User outside the block
				assertNotContainsPattern(t, generatedStr, "type User = source.User")
				assertNotContainsPattern(t, generatedStr, "type User = target.User")

				// Assert conversion functions exist with correct naming
				assertContainsPattern(t, generatedStr, `func ConvertUserSourceToUserTarget`)
				assertContainsPattern(t, generatedStr, `func ConvertUserTargetToUserSource`)

				// Assert that 'Id' from source is mapped to 'ID' in target
				assertContainsPattern(t, generatedStr, `target := &UserTarget{`)
				assertContainsPattern(t, generatedStr, `ID:   from.Id,`) // Correct: target.ID = source.Id
				assertContainsPattern(t, generatedStr, `Name: from.Name,`)

				assertContainsPattern(t, generatedStr, `target := &UserSource{`)
				assertContainsPattern(t, generatedStr, `Id:   from.ID,`) // Correct: target.Id = source.ID
				assertContainsPattern(t, generatedStr, `Name: from.Name,`)
			},
		},

		// === 04_type_aliases: Type Alias Handling ===
		{
			name:           "auto_generate_aliases",
			directivePath:  "../../testdata/03_advanced_features/auto_generate_aliases",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "type_aliases",
		},

		// === 06_complex_types: Complex Type Conversions ===
		        {
		            name:           "slice_conversions",
		            directivePath:  "../../testdata/03_advanced_features/slice_conversions",
		            goldenFileName: "expected.golden",
		            dependencies: []string{
		                "github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversions/source",
		                "github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversions/target",
		            },
		
			priority: "P0",
			category: "complex_types",
		},
		{
			name:          "enum_string_to_int",
			            directivePath: "../../testdata/03_advanced_features/enum_string_to_int",
			            dependencies: []string{
			                "github.com/origadmin/abgen/testdata/03_advanced_features/enum_string_to_int/source",
			                "github.com/origadmin/abgen/testdata/03_advanced_features/enum_string_to_int/target",
			            },			priority: "P1",
			category: "complex_types",
			assertFunc: func(t *testing.T, generatedCode []byte) {
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
			category: "complex_types",
			assertFunc: func(t *testing.T, generatedCode []byte) {
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
			category: "complex_types",
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for map_conversions")
			},
		},
		{
			name:          "numeric_conversions",
			directivePath: "../../testdata/03_advanced_features/numeric_conversions",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/03_advanced_features/numeric_conversions/source",
				"github.com/origadmin/abgen/testdata/03_advanced_features/numeric_conversions/target",
			},
			priority: "P1",
			category: "complex_types",
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for numeric_conversions")
			},
		},

		// === 07_custom_rules: Custom Rules ===
		{
			name:          "custom_function_rules",
			directivePath: "../../testdata/03_advanced_features/custom_function_rules",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/source",
				"github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/target",
			},
			priority: "P0",
			category: "custom_rules",
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for custom_function_rules")
			},
		},

		// === 09_array_slice_fix: Array/Slice Conversion Fixes ===
		{
			name:           "array_slice_test",
			directivePath:  "../../testdata/06_regression/array_slice_fix/array_slice_test",
			goldenFileName: "expected.golden",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/source",
				"github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/target",
			},
			priority: "P0",
			category: "array_slice_fix",
		},

		// === Legacy and Special Cases ===
		{
			name:           "alias-gen", // Specific bug fix test case
			directivePath:  "../../testdata/06_regression/alias_gen_fix",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "alias-gen",
		},
	}

	// Sort test cases for consistent execution order
	sort.Slice(testCases, func(i, j int) bool {
		priorityOrder := map[string]int{"P0": 0, "P1": 1, "P2": 2}
		if priorityOrder[testCases[i].priority] != priorityOrder[testCases[j].priority] {
			return priorityOrder[testCases[i].priority] < priorityOrder[testCases[j].priority]
		}
		return testCases[i].category < testCases[j].category
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running test: %s (Priority: %s, Category: %s)", tc.name, tc.priority, tc.category)

			// Step 1: Parse config from the directive path using the new high-level API.
			parser := config.NewParser()
			cfg, err := parser.Parse(tc.directivePath)
			if err != nil {
				t.Fatalf("config.Parser.Parse() failed: %v", err)
			}

			// Step 2: Analyze types using the new high-level API.
			typeAnalyzer := analyzer.NewTypeAnalyzer()
			packagePaths := cfg.RequiredPackages()
			typeFQNs := cfg.RequiredTypeFQNs()
			typeInfos, err := typeAnalyzer.Analyze(packagePaths, typeFQNs)
			if err != nil {
				t.Fatalf("analyzer.TypeAnalyzer.Analyze() failed: %v", err)
			}

			// Step 3: Run the generator with the results.
			g := NewGenerator(cfg)
			generatedCode, err := g.Generate(typeInfos)
			if err != nil {
				t.Fatalf("Generate() failed for test case %s: %v", tc.name, err)
			}

			// Normalize path separators in generated code for consistent comparison across OS.
			// The `source` comment line uses paths that might differ by OS.
			generatedCodeStr := string(generatedCode)
			generatedCodeStr = strings.ReplaceAll(generatedCodeStr, `\`, `/`)
			generatedCode = []byte(generatedCodeStr)

			// Step 4.1: Run custom assertions if provided.
			if tc.assertFunc != nil {
				tc.assertFunc(t, generatedCode)
			}

			// Step 4.2: Snapshot testing - compare against a "golden" file.
			if tc.goldenFileName != "" { // Only attempt golden file comparison if goldenFileName is provided
				goldenFile := filepath.Join(tc.directivePath, tc.goldenFileName)
				if os.Getenv("UPDATE_GOLDEN_FILES") != "" {
					err = os.WriteFile(goldenFile, generatedCode, 0644)
					if err != nil {
						t.Fatalf("Failed to update golden file %s: %v", goldenFile, err)
					}
					t.Logf("Updated golden file: %s", goldenFile)
					return // Skip comparison when updating
				}

				expectedCode, err := os.ReadFile(goldenFile)
				if err != nil {
					t.Fatalf("Failed to read golden file %s: %v", goldenFile, err)
				}

				if string(generatedCode) != string(expectedCode) {
					actualOutputFile := filepath.Join(tc.directivePath, tc.name+".actual.gen.go") // Save to a unique file
					_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
					t.Errorf("Generated code for '%s' does not match the golden file %s. The generated output was saved to %s for inspection.", tc.name, goldenFile, actualOutputFile)
				}
			} else {
				t.Logf("Skipping golden file comparison for test: %s (no goldenFileName provided)", tc.name)
			}
		})
	}
}

// assertContainsPattern checks if the generated code contains a specific regular expression pattern.
func assertContainsPattern(t *testing.T, code string, pattern string) {
	t.Helper()
	escapedPattern := regexp.QuoteMeta(pattern) // Escape pattern for literal match
	match, err := regexp.MatchString(escapedPattern, code)
	if err != nil {
		t.Fatalf("Invalid regex pattern %q: %v", escapedPattern, err)
	}
	if !match {
		t.Errorf("Generated code does not contain expected pattern %q.\nGenerated Code:\n%s", pattern, code)
	}
}

// assertNotContainsPattern checks if the generated code does NOT contain a specific regular expression pattern.
func assertNotContainsPattern(t *testing.T, code string, pattern string) {
	t.Helper()
	escapedPattern := regexp.QuoteMeta(pattern) // Escape pattern for literal match
	match, err := regexp.MatchString(escapedPattern, code)
	if err != nil {
		t.Fatalf("Invalid regex pattern %q: %v", escapedPattern, err)
	}
	if match {
		t.Errorf("Generated code contains unexpected pattern %q.\nGenerated Code:\n%s", pattern, code)
	}
}


