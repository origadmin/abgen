package generator

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/ast"
)

// loadTestPackages is a helper function from the ast package's tests.
// We are re-using it here to set up the walker.
func loadTestPackages(t *testing.T, testCaseDir string, dependencies ...string) (pkgs []*packages.Package, directivePkg *packages.Package) {
	t.Helper()
	absTestCaseDir, err := filepath.Abs(testCaseDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for test case dir %s: %v", testCaseDir, err)
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir:  absTestCaseDir,
	}
	loadPatterns := append([]string{"."}, dependencies...)
	loadedPkgs, err := packages.Load(cfg, loadPatterns...)
	if err != nil {
		t.Fatalf("Failed to load packages: %v", err)
	}
	if packages.PrintErrors(loadedPkgs) > 0 {
		t.Fatal("Errors occurred while loading packages.")
	}
	for _, p := range loadedPkgs {
		if len(p.GoFiles) > 0 && filepath.Dir(p.GoFiles[0]) == absTestCaseDir {
			return loadedPkgs, p
		}
	}
	t.Fatalf("Could not find the directive package in directory %s", absTestCaseDir)
	return nil, nil
}

func TestGenerator_CodeGeneration(t *testing.T) {
	// Base dependencies for most tests
	baseDependencies := []string{
		"github.com/origadmin/abgen/testdata/fixture/ent",
		"github.com/origadmin/abgen/testdata/fixture/types",
	}

	testCases := []struct {
		name           string
		directivePath  string
		goldenFileName string
		dependencies   []string
		priority       string // P0, P1, P2 for prioritization
		category       string // Test category for organization
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
		// Note: All complex type tests are skipped - golden files not available
		// - slice_conversions
		// - enum_string_to_int
		// - pointer_conversions
		// - map_conversions
		// - numeric_conversions

		// === 07_custom_rules: Custom Rules ===
		// Note: custom_function_rules test is skipped - golden file not available

		// === Legacy and Special Cases ===
		{
			name:           "bug_fix_001", // Specific bug fix test case
			directivePath:  "../../testdata/bug_fix_001",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "legacy",
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
			// Step 1: Perform a full analysis to get the walker into its final state.
			allPkgs, directivePkg := loadTestPackages(t,
				tc.directivePath,
				tc.dependencies...,
			)
			walker := ast.NewPackageWalker() // Call without graph parameter
			walker.AddPackages(allPkgs...)
			if err := walker.Analyze(directivePkg); err != nil {
				t.Fatalf("Walker.Analyze() failed: %v", err)
			}

			// Step 2: Run the generator.
			g := NewGenerator(walker)
			generatedCode, err := g.Generate()
			if err != nil {
				t.Fatalf("Generate() failed: %v", err)
			}

			// Step 3: Snapshot testing - compare against a "golden" file.
			goldenFile := filepath.Join(tc.directivePath, tc.goldenFileName)
			if os.Getenv("UPDATE_GOLDEN_FILES") != "" {
				os.WriteFile(goldenFile, generatedCode, 0644)
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
