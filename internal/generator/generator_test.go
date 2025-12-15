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
		dependencies   []string                 // These are now hints for directive parsing, not direct load patterns
		priority       string                   // P0, P1, P2 for prioritization
		category       string                   // Test category for organization
		assertFunc     func(*testing.T, []byte) // Custom assertion function for detailed checks
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
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for simple_struct_conversion")
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
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for package_level_conversion")
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
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for oneway_conversion")
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
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for id_to_id_field_conversion")
			},
		},
		{
			name:         "simple_bilateral",
			directivePath: "../../testdata/02_basic_conversions/simple_bilateral",
			dependencies:  baseDependencies,
			priority:      "P0",
			category:      "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for simple_bilateral")
			},
		},
		{
			name:         "standard_trilateral",
			directivePath: "../../testdata/02_basic_conversions/standard_trilateral",
			dependencies:  baseDependencies,
			priority:      "P0",
			category:      "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for standard_trilateral")
			},
		},
		{
			name:          "field_ignore_remap",
			directivePath: "../../testdata/02_basic_conversions/field_ignore_remap",
			goldenFileName: "expected.golden", // Will generate this later
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/field_ignore_remap/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/field_ignore_remap/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte) {
				generatedStr := string(generatedCode)
				// Assertions for ConvertUserToUserDTO
				assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO`)
				// Ignored fields should NOT be present in the output
				assertNotContainsPattern(t, generatedStr, `Password:`)
				// Assert that CreatedAt is not directly assigned to CreatedDate (because CreatedAt is ignored)
				assertNotContainsPattern(t, generatedStr, `CreatedAt:`)
				// Remapped fields should be present with new names
				assertContainsPattern(t, generatedStr, `FullName: from.Name,`)
				assertContainsPattern(t, generatedStr, `UserEmail: from.Email,`)
				// Non-remapped, non-ignored fields should be present with original names (if types match)
				assertContainsPattern(t, generatedStr, `Id: from.ID,`)

				// Handle time.Time fields.
				// Source.CreatedAt is ignored, so target.CreatedDate should not come from it.
				// Source.UpdatedAt is not ignored, and target.LastUpdate matches name.
				assertContainsPattern(t, generatedStr, `LastUpdate: ConvertTimeToTime\(from.UpdatedAt\),`) // Assuming ConvertTimeToTime is a generated helper
				assertNotContainsPattern(t, generatedStr, `CreatedDate: ConvertTimeToTime\(from.CreatedAt\),`) // CreatedAt is ignored
				// If target.CreatedDate is not remapped and not mapped from an ignored field,
				// it should be initialized to its zero value or left out if Go's default is acceptable.
				// Since source.CreatedAt is ignored, target.CreatedDate won't be explicitly mapped from it.
				// We expect it to be either absent or implicitly zero-valued.
				assertNotContainsPattern(t, generatedStr, `CreatedDate:`) // It shouldn't be explicitly assigned if no source
			},
		},


		// === 03_advanced_features: Advanced Feature Tests ===
		{
			name:           "auto_generate_aliases",
			directivePath:  "../../testdata/03_advanced_features/auto_generate_aliases",
			goldenFileName: "expected.golden",
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
			assertFunc: func(t *testing.T, generatedCode []byte) {
				generatedStr := string(generatedCode)
				// Check forward conversion (User -> UserCustom)
				assertContainsPattern(t, generatedStr, `func ConvertUserToUserCustom`)
				assertContainsPattern(t, generatedStr, `Status: IntStatusToString\(from.Status\),`)

				// Check reverse conversion (UserCustom -> User)
				assertContainsPattern(t, generatedStr, `func ConvertUserCustomToUser`)
				assertContainsPattern(t, generatedStr, `Status: StringStatusToInt\(from.Status\),`)
			},
		},
		{
			name:           "slice_conversions",
			directivePath:  "../../testdata/03_advanced_features/slice_conversions",
			goldenFileName: "expected.golden",
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
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for enum_string_to_int")
			},

			// Write success file for inspection (only for 02_basic_conversions)

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
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for pointer_conversions")
			},

			// Write success file for inspection (only for 02_basic_conversions)

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
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for map_conversions")
			},

			// Write success file for inspection (only for 02_basic_conversions)

		},
		{
			name:          "numeric_conversions",
			directivePath: "../../testdata/03_advanced_features/numeric_conversions",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/03_advanced_features/numeric_conversions/source",
				"github.com/origadmin/abgen/testdata/03_advanced_features/numeric_conversions/target",
			},
			priority: "P1",
			category: "advanced_features",
			assertFunc: func(t *testing.T, generatedCode []byte) {
				t.Log("TODO: Add specific assertions for numeric_conversions")
			},

			// Write success file for inspection (only for 02_basic_conversions)

		},

		// === 06_regression: Regression Tests ===
		{
			name:           "array_slice_test",
			directivePath:  "../../testdata/06_regression/array_slice_fix/array_slice_test",
			goldenFileName: "expected.golden",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/source",
				"github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/target",
			},
			priority: "P0",
			category: "regression",
		},

		{
			name:           "alias-gen", // Specific bug fix test case
			directivePath:  "../../testdata/06_regression/alias_gen_fix",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "regression",
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
		// Extract the numeric prefix from the directory path to ensure the stage number
		// in the test name matches the physical directory structure.
		// e.g., extracts "03" from "../../testdata/03_advanced_features/..."
		var stagePrefix string
		pathParts := strings.Split(tc.directivePath, "/")
		for _, part := range pathParts {
			if len(part) > 2 && part[2] == '_' && part[0] >= '0' && part[0] <= '9' && part[1] >= '0' && part[1] <= '9' {
				stagePrefix = part[:2]
				break
			}
		}

		// Prepend the stage number to the test name for clear, always-visible grouping.
		testNameWithStage := fmt.Sprintf("%s_%s/%s", stagePrefix, tc.category, tc.name)
		t.Run(testNameWithStage, func(t *testing.T) {
			t.Logf("Running test: %s (Priority: %s, Category: %s)", tc.name, tc.priority, tc.category)

			// Cleanup any previously generated files in the testdata directory to ensure a clean slate.
			// This prevents "redeclared in this block" errors when running tests repeatedly.
			cleanTestFiles(t, tc.directivePath)
			defer cleanTestFiles(t, tc.directivePath)


			// Step 1: Parse config from the directive path using the new high-level API.
			parser := config.NewParser()
			cfg, err := parser.Parse(tc.directivePath)
			if err != nil {
				t.Fatalf("config.Parser.Parse() failed: %v", err)
			}

			// Step 2: Analyze types using the new high-level API.
			typeAnalyzer := analyzer.NewTypeAnalyzer() // Pass the root dir for analysis context
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
				// Create a subtest to capture failures more clearly
				t.Run(tc.name+"_Assertions", func(st *testing.T) {
					tc.assertFunc(st, generatedCode)
					if st.Failed() {
						actualOutputFile := filepath.Join(tc.directivePath, "failed.actual.gen.go")
						_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
						st.Logf("Assertion failed for '%s'. Generated output saved to %s for inspection.", tc.name, actualOutputFile)
					} else {
						actualOutputFile := filepath.Join(tc.directivePath, "success.actual.gen.go")
						_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
					}
				})
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
					actualOutputFile := filepath.Join(tc.directivePath, "failed.actual.gen.go") // Save to a unique file
					_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
					t.Errorf("Generated code for '%s' does not match the golden file %s. The generated output was saved to %s for inspection.", tc.name, goldenFile, actualOutputFile)
				}
			} else {
				t.Logf("Skipping golden file comparison for test: %s (no goldenFileName provided)", tc.name)
			}
		})
	}
}

// TestDefaultDirectionBehavior tests that the default direction is 'both' when not explicitly specified
func TestDefaultDirectionBehavior(t *testing.T) {
	slog.Debug("Starting TestDefaultDirectionBehavior")

	// Test case: simple_struct should generate both directions by default
	testPath := "../../testdata/02_basic_conversions/simple_struct"

	slog.Debug("Testing directory", "path", testPath)

	// Parse configuration from the test directory
	parser := config.NewParser()
	config, err := parser.Parse(testPath)
	if err != nil {
		slog.Error("Failed to parse configuration", "error", err)
		t.Fatalf("Failed to parse configuration: %v", err)
	}

	slog.Debug("Parsed configuration", "rules_count", len(config.ConversionRules))

	// Assert that we have at least one conversion rule
	if len(config.ConversionRules) == 0 {
		t.Fatal("Expected at least one conversion rule, got none")
	}

	// Get the first conversion rule for testing
	rule := config.ConversionRules[0]
	slog.Debug("First rule", "source", rule.SourceType, "target", rule.TargetType, "direction", rule.Direction)

	// The key assertion: direction should default to 'both'
	if rule.Direction != "both" {
		slog.Error("Direction should default to 'both'", "actual", rule.Direction)
		t.Errorf("Expected direction to be 'both', got '%s'", rule.Direction)
	}

	slog.Debug("Direction assertion passed")
	slog.Debug("TestDefaultDirectionBehavior completed successfully")
}

func cleanTestFiles(t *testing.T, dir string) {
	files, err := filepath.Glob(filepath.Join(dir, "*.actual.gen.go"))
	if err != nil {
		t.Fatalf("Failed to glob for generated files in %s: %v", dir, err)
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			// Change from t.Fatalf to t.Logf to prevent flaky tests on Windows due to file locks.
			t.Logf("Warning: Failed to remove old generated file %s: %v", f, err)
		}
	}
	// Also remove the failed output file if it exists
	failedFile := filepath.Join(dir, "failed.actual.gen.go")
	if _, err := os.Stat(failedFile); err == nil {
		if err := os.Remove(failedFile); err != nil {
			// Change from t.Fatalf to t.Logf
			t.Logf("Warning: Failed to remove old failed output file %s: %v", failedFile, err)
		}
	}
}

// assertContainsPattern checks if the generated code contains a specific regular expression pattern.
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

// assertNotContainsPattern checks if the generated code does NOT contain a specific regular expression pattern.
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
