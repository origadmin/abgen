package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/origadmin/abgen/internal/analyzer"
	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/generator"
)

// TestGenerator_Golden is the new integration test.
// It iterates through subdirectories in a root testdata folder.
// Each subdirectory is a test case with its own directives.go and an expected.golden file.
func TestGenerator_Golden(t *testing.T) {
	testdataRoot := "../../testdata" // Adjust this path if necessary

	dirs, err := os.ReadDir(testdataRoot)
	if err != nil {
		t.Fatalf("Failed to read testdata root directory: %v", err)
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		testCaseDir := filepath.Join(testdataRoot, dir.Name())
		// Look for sub-directories which are the actual test cases
		subDirs, err := os.ReadDir(testCaseDir)
		if err != nil {
			t.Logf("Could not read subdirectories in %s, skipping: %v", testCaseDir, err)
			continue
		}

		for _, subDir := range subDirs {
			if !subDir.IsDir() {
				continue
			}

			casePath := filepath.Join(testCaseDir, subDir.Name())
			t.Run(subDir.Name(), func(t *testing.T) {
				t.Parallel() // Run test cases in parallel

				directivesFile := filepath.Join(casePath, "directives.go")
				goldenFile := filepath.Join(casePath, "expected.golden")
				if _, err := os.Stat(directivesFile); os.IsNotExist(err) {
					t.Skipf("No directives.go file found in %s, skipping.", casePath)
				}
				if _, err := os.Stat(goldenFile); os.IsNotExist(err) {
					t.Skipf("No expected.golden file found in %s, skipping.", casePath)
				}
				// 1. Parse configuration using the new high-level API.
				parser := config.NewParser()
				cfg, err := parser.Parse(casePath) // Pass the specific test case path directly.

				if err != nil {
					t.Fatalf("config.Parser.Parse() failed for %s: %v", casePath, err)
				}

				// 2. Analyze types using the new high-level API.
				typeAnalyzer := analyzer.NewTypeAnalyzer()
				packagePaths := cfg.RequiredPackages()
				typeFQNs := cfg.RequiredTypeFQNs()
				typeInfos, err := typeAnalyzer.Analyze(packagePaths, typeFQNs)
				if err != nil {
					t.Fatalf("analyzer.TypeAnalyzer.Analyze() failed for %s: %v", casePath, err)
				}
				// 3. Run the generator with the results.
				gen := generator.NewGenerator(cfg) // NewGenerator only needs the Config
				generatedBytes, err := gen.Generate(typeInfos)
				if err != nil {
					t.Fatalf("Generator failed: %v", err)
				}
				if os.Getenv("UPDATE_GOLDEN_FILES") != "" {
					if err := os.WriteFile(goldenFile, generatedBytes, 0644); err != nil {
						t.Fatalf("Failed to update golden file %s: %v", goldenFile, err)
					}
					t.Logf("Updated golden file: %s", goldenFile)
					return // Skip comparison when updating
				}
				goldenBytes, err := os.ReadFile(goldenFile)
				if err != nil {
					t.Fatalf("Failed to read golden file: %v", err)
				}
				// 9. Compare generated output with the golden file
				// Normalize line endings for comparison
				generatedStr := strings.ReplaceAll(string(generatedBytes), "\r\n", "\n")
				goldenStr := strings.ReplaceAll(string(goldenBytes), "\r\n", "\n")

				if generatedStr != goldenStr {
					dmp := diffmatchpatch.New() // Fixed: use imported diffmatchpatch
					diffs := dmp.DiffMain(goldenStr, generatedStr, false)
					t.Errorf("Generated code does not match golden file %s.\n\n%s", goldenFile, dmp.DiffPrettyText(diffs)) // Added goldenFile to error message
				}
			})
		}

	}
}
