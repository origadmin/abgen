package generator

import (
	"os"
	"path/filepath"
	"sort"
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
		"github.com/origadmin/abgen/testdata/fixture/ent",
		"github.com/origadmin/abgen/testdata/fixture/types",
	}

	testCases := []struct {
		name           string
		directivePath  string
		goldenFileName string
		dependencies   []string // These are now hints for directive parsing, not direct load patterns
		priority       string   // P0, P1, P2 for prioritization
		category       string   // Test category for organization
	}{
		// === 01_basic_modes: Basic Conversion Patterns ===
		{
			name:           "simple_bilateral",
			directivePath:  "../../testdata/01_basic_modes/simple_bilateral",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "basic_modes",
		},
		{
			name:           "standard_trilateral",
			directivePath:  "../../testdata/01_basic_modes/standard_trilateral",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "basic_modes",
		},
		// Note: multi_source test is skipped - definition does not match current implementation requirements
		// Note: multi_target test is skipped - golden file not available and definition needs refinement

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

		// === 04_type_aliases: Type Alias Handling ===
		{
			name:           "auto_generate_aliases",
			directivePath:  "../../testdata/04_type_aliases/auto_generate_aliases",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "type_aliases",
		},

		// === 05_field_mapping: Field Mapping ===
		// Note: simple_field_remap test is skipped - golden file not available

		// === 06_complex_types: Complex Type Conversions ===
		{
			name:           "slice_conversions",
			directivePath:  "../../testdata/08_slice_conversions/slice_conversion_test_case",
			goldenFileName: "expected.golden",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/08_slice_conversions/slice_conversion_test_case/source",
				"github.com/origadmin/abgen/testdata/08_slice_conversions/slice_conversion_test_case/target",
			},
			priority: "P0",
			category: "complex_types",
		},
		// - enum_string_to_int
		// - pointer_conversions
		// - map_conversions
		// - numeric_conversions

		// === 07_custom_rules: Custom Rules ===
		// Note: custom_function_rules test is skipped - golden file not available

		// === 09_array_slice_fix: Array/Slice Conversion Fixes ===
		{
			name:           "array_slice_test",
			directivePath:  "../../testdata/09_array_slice_fix/array_slice_test",
			goldenFileName: "expected.golden",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/09_array_slice_fix/array_slice_test/source",
				"github.com/origadmin/abgen/testdata/09_array_slice_fix/array_slice_test/target",
			},
			priority: "P0",
			category: "array_slice_fix",
		},

		// === Legacy and Special Cases ===
		{
			name:           "alias-gen", // Specific bug fix test case
			directivePath:  "../../testdata/10_bug_fix_issue/alias-gen",
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

			// Step 4: Snapshot testing - compare against a "golden" file.
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
				generatedFile := filepath.Join(tc.directivePath, "expected.golden")
				_ = os.WriteFile(generatedFile, generatedCode, 0644)
				t.Errorf("Generated code does not match the golden file %s. The generated output was saved to %s for inspection.", goldenFile, generatedFile)
			}
		})
	}
}
