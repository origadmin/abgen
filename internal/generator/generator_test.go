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
	testCases := []struct {
		name          string
		directivePath string
		goldenFileName string
		dependencies  []string
	}{
		{
			name:          "p01_basic",
			directivePath: "../../testdata/directives/p01_basic",
			goldenFileName: "p01_basic.golden",
			dependencies:  []string{
				"github.com/origadmin/abgen/testdata/fixture/ent",
				"github.com/origadmin/abgen/testdata/fixture/types",
			},
		},
		{
			name:          "p02_alias",
			directivePath: "../../testdata/directives/p02_alias",
			goldenFileName: "p02_alias.golden",
			dependencies:  []string{
				"github.com/origadmin/abgen/testdata/fixture/ent",
				"github.com/origadmin/abgen/testdata/fixture/types",
			},
		},
		{
			name:          "p03_remap",
			directivePath: "../../testdata/directives/p03_remap",
			goldenFileName: "p03_remap.golden",
			dependencies:  []string{
				"github.com/origadmin/abgen/testdata/fixture/ent",
				"github.com/origadmin/abgen/testdata/fixture/types",
			},
		},
		{
			name:          "p04_slice",
			directivePath: "../../testdata/directives/p04_slice",
			goldenFileName: "p04_slice.golden",
			dependencies:  []string{
				"github.com/origadmin/abgen/testdata/fixture/ent",
				"github.com/origadmin/abgen/testdata/fixture/types",
			},
		},
		{
			name:          "p05_enum",
			directivePath: "../../testdata/directives/p05_enum",
			goldenFileName: "p05_enum.golden",
			dependencies:  []string{
				"github.com/origadmin/abgen/testdata/fixture/ent",
				"github.com/origadmin/abgen/testdata/fixture/types",
			},
		},
		{
			name:          "p06_custom",
			directivePath: "../../testdata/directives/p06_custom",
			goldenFileName: "p06_custom.golden",
			dependencies:  []string{
				"github.com/origadmin/abgen/testdata/fixture/ent",
				"github.com/origadmin/abgen/testdata/fixture/types",
			},
		},
		{
			name:          "example_trilateral", // Complete trilateral conversion example
			directivePath: "../../testdata/directives/example_trilateral",
			goldenFileName: "example_trilateral.golden",
			dependencies:  []string{
				"github.com/origadmin/abgen/testdata/fixture/ent",
				"github.com/origadmin/abgen/testdata/fixture/types",
			},
		},
		{
			name:          "bug_fix_001", // This is a specific bug fix test case
			directivePath: "../../testdata/directives/bug_fix_001",
			goldenFileName: "bug_fix_001.golden",
			dependencies:  []string{
				"github.com/origadmin/abgen/testdata/fixture/ent",
				"github.com/origadmin/abgen/testdata/fixture/types",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
			goldenFile := filepath.Join("../../testdata/golden", tc.goldenFileName)
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
