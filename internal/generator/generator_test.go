package generator

import (
	"os"
	"path/filepath"
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

	// Sort test cases by priority and category for consistent execution order
	sortedTestCases := make([]struct {
		name           string
		directivePath  string
		goldenFileName string
		dependencies   []string
		priority       string
		category       string
	}, len(testCases))
	copy(sortedTestCases, testCases)

	// Simple sort: P0 first, then P1, then P2; within each priority, sort by category
	for i := 0; i < len(sortedTestCases); i++ {
		for j := i + 1; j < len(sortedTestCases); j++ {
			priorityOrder := map[string]int{"P0": 0, "P1": 1, "P2": 2}
			if priorityOrder[sortedTestCases[i].priority] > priorityOrder[sortedTestCases[j].priority] {
				sortedTestCases[i], sortedTestCases[j] = sortedTestCases[j], sortedTestCases[i]
			} else if priorityOrder[sortedTestCases[i].priority] == priorityOrder[sortedTestCases[j].priority] &&
				sortedTestCases[i].category > sortedTestCases[j].category {
				sortedTestCases[i], sortedTestCases[j] = sortedTestCases[j], sortedTestCases[i]
			}
		}
	}

	for _, tc := range sortedTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running test: %s (Priority: %s, Category: %s)", tc.name, tc.priority, tc.category)

			// Step 1: Initialize analyzer.PackageWalker
			walker := analyzer.NewPackageWalker()

			// Step 2: Load initial package to discover directives
			initialPkg, err := walker.LoadInitialPackage(tc.directivePath)
			if err != nil {
				t.Fatalf("Failed to load initial package from %s: %v", tc.directivePath, err)
			}

			// Step 3: Discover directives from the initial package
			directiveParser := config.NewDirectiveParser()
			directives, err := directiveParser.DiscoverDirectives(initialPkg)
			if err != nil {
				t.Fatalf("Failed to discover directives in %s: %v", tc.directivePath, err)
			}

			// Step 4: Extract dependencies from directives.
			// The 'dependencies' field in the test case can be used as additional hints if needed,
			// but the primary source should be the directives themselves.
			// For now, we'll just use the dependencies extracted from directives.
			dependencyPaths := directiveParser.ExtractDependencies(directives)

			// Step 5: Load full graph including dependencies
			// The LoadFullGraph method will ensure all necessary packages are loaded and analyzed.
			_, err = walker.LoadFullGraph(tc.directivePath, dependencyPaths...)
			if err != nil {
				t.Fatalf("Failed to load full package graph for %s with dependencies %v: %v", tc.directivePath, dependencyPaths, err)
			}

			// Step 6: Parse directives into a RuleSet
			ruleParser := config.NewRuleParser()
			err = ruleParser.ParseDirectives(directives, initialPkg) // Corrected: Call ParseDirectives with package context
			if err != nil {
				t.Fatalf("Failed to parse directives into RuleSet for %s: %v", tc.directivePath, err)
			}
			ruleSet := ruleParser.GetRuleSet() // Corrected: Get RuleSet after parsing

			// Step 7: Run the generator.
			// The NewGenerator should now take the analyzer.PackageWalker and the config.RuleSet.
			g := NewGenerator(walker, ruleSet)
			generatedCode, err := g.Generate()
			if err != nil {
				t.Fatalf("Generate() failed for test case %s: %v", tc.name, err)
			}

			// Step 8: Snapshot testing - compare against a "golden" file.
			goldenFile := filepath.Join(tc.directivePath, tc.goldenFileName)
			if os.Getenv("UPDATE_GOLDEN_FILES") != "" {
				err = os.WriteFile(goldenFile, generatedCode, 0644)
				if err != nil {
					t.Fatalf("Failed to update golden file %s: %v", goldenFile, err)
				}
				t.Logf("Updated golden file: %s", goldenFile)
			}

			expectedCode, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("Failed to read golden file %s: %v", goldenFile, err)
			}

			if string(generatedCode) != string(expectedCode) {
				t.Errorf("Generated code does not match the golden file %s.\n---EXPECTED---\n%s\n---GOT---\n%s",
					goldenFile, string(expectedCode), string(generatedCode))
			}
		})
	}
}
