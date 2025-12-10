package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/origadmin/abgen/internal/abgen"
	"github.com/sergi/go-diff/diffmatchpatch"
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
					t.Skip("No directives.go file found, skipping.")
				}
				if _, err := os.Stat(goldenFile); os.IsNotExist(err) {
					t.Skip("No expected.golden file found, skipping.")
				}

				// 2. Run the generator
				generatedBytes, err := abgen.Generate(casePath)
				if err != nil {
					t.Fatalf("Generator failed: %v", err)
				}

				// 3. Read the golden file
				goldenBytes, err := os.ReadFile(goldenFile)
				if err != nil {
					t.Fatalf("Failed to read golden file: %v", err)
				}

				// 4. Compare generated output with the golden file
				// Normalize line endings for comparison
				generatedStr := strings.ReplaceAll(string(generatedBytes), "\r\n", "\n")
				goldenStr := strings.ReplaceAll(string(goldenBytes), "\r\n", "\n")

				if generatedStr != goldenStr {
					dmp := diffmatchpatch.New()
					diffs := dmp.DiffMain(goldenStr, generatedStr, false)
					t.Errorf("Generated code does not match golden file.\n\n%s", dmp.DiffPrettyText(diffs))
				}
			})
		}
	}
}