package ast

import (
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/types"
)

// loadTestPackages loads the Go packages required for a specific test case.
// It takes the root directory of the test case (e.g., "../../testdata/directives/p01_basic")
// and a list of dependent package paths to load.
// It returns all loaded packages and the specific directive package.
func loadTestPackages(t *testing.T, testCaseDir string, dependencies ...string) (pkgs []*packages.Package, directivePkg *packages.Package) {
	t.Helper()
	absTestCaseDir, err := filepath.Abs(testCaseDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for test case dir %s: %v", testCaseDir, err)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir:  absTestCaseDir, // Set the working directory for package loading
	}

	// The patterns to load: the directive package itself ("./") and its dependencies.
	loadPatterns := append([]string{"."}, dependencies...)

	loadedPkgs, err := packages.Load(cfg, loadPatterns...)
	if err != nil {
		t.Fatalf("Failed to load packages: %v", err)
	}

	if packages.PrintErrors(loadedPkgs) > 0 {
		t.Fatal("Errors occurred while loading packages.")
	}

	if len(loadedPkgs) == 0 {
		t.Fatal("Expected to load at least one package, but got none.")
	}

	// Find the directive package, which is the one loaded from the current directory (".").
	for _, p := range loadedPkgs {
		// A package loaded from "." will have its directory as its ID if it's not in GOPATH.
		// We check if any of its Go files are in the target directory.
		if len(p.GoFiles) > 0 && filepath.Dir(p.GoFiles[0]) == absTestCaseDir {
			return loadedPkgs, p
		}
	}

	t.Fatalf("Could not find the directive package in directory %s", absTestCaseDir)
	return nil, nil // Should be unreachable
}

func findPackage(pkgs []*packages.Package, pkgPath string) *packages.Package {
	for _, pkg := range pkgs {
		if pkg.PkgPath == pkgPath {
			return pkg
		}
	}
	return nil
}

func TestWalker_P01BasicDirectives(t *testing.T) {
	// Avoid setting global logger state in tests. Use t.Logf for debugging if needed.
	// slog.SetLogLoggerLevel(slog.LevelDebug)

	// Load packages for p01_basic test case
	allPkgs, directivePkg := loadTestPackages(t,
		"../../testdata/directives/p01_basic",
		"github.com/origadmin/abgen/testdata/fixture/ent",
		"github.com/origadmin/abgen/testdata/fixture/types",
	)

	graph := make(types.ConversionGraph)
	walker := NewPackageWalker(graph)
	walker.AddPackages(allPkgs...) // Add all loaded packages to the walker for resolution

	err := walker.WalkPackage(directivePkg) // Walk the directive package to extract configs
	if err != nil {
		t.Fatalf("Walker.WalkPackage() failed: %v", err)
	}

	t.Run("PackagePairDirective", func(t *testing.T) {
		if len(walker.PackageConfigs) != 1 {
			t.Fatalf("Expected 1 PackageConversionConfig, got %d", len(walker.PackageConfigs))
		}
		pkgCfg := walker.PackageConfigs[0]

		// Define constants for package paths to avoid magic strings and improve maintainability.
		const (
			entPkgPath   = "github.com/origadmin/abgen/testdata/fixture/ent"
			typesPkgPath = "github.com/origadmin/abgen/testdata/fixture/types"
		)

		// Helper to create fully-qualified names for cleaner test assertions.
		fqn := func(pkgPath, typeName, fieldName string) string {
			return pkgPath + "." + typeName + "#" + fieldName
		}

		if pkgCfg.SourcePackage != entPkgPath {
			t.Errorf("Expected SourcePackage %q, got %q", entPkgPath, pkgCfg.SourcePackage)
		}
		if pkgCfg.TargetPackage != typesPkgPath {
			t.Errorf("Expected TargetPackage %q, got %q", typesPkgPath, pkgCfg.TargetPackage)
		}
		if pkgCfg.SourceSuffix != "" {
			t.Errorf("Expected SourceSuffix \"\", got %q", pkgCfg.SourceSuffix)
		}
		if pkgCfg.TargetSuffix != "PB" { // This comes from convert:target:suffix="PB"
			t.Errorf("Expected TargetSuffix 'PB', got %q", pkgCfg.TargetSuffix)
		}
		// This comes from convert:direction="oneway"
		if pkgCfg.Direction != "oneway" { 
			t.Errorf("Expected Direction 'oneway', got %q", pkgCfg.Direction)
		}

		// Test RemapFields
		expectedRemaps := map[string]string{fqn(entPkgPath, "User", "ID"): "Id"}
		if !reflect.DeepEqual(pkgCfg.RemapFields, expectedRemaps) {
			t.Errorf("Expected remap fields %v, got %v", expectedRemaps, pkgCfg.RemapFields)
		}

		// Test IgnoreFields
		expectedIgnores := map[string]bool{
			fqn(entPkgPath, "User", "Password"):  true,
			fqn(entPkgPath, "User", "Salt"):      true,
			fqn(entPkgPath, "User", "CreatedAt"): true,
			fqn(entPkgPath, "User", "UpdatedAt"): true,
			fqn(entPkgPath, "User", "Edges"):     true,
			fqn(entPkgPath, "User", "Gender"):    true,
		}
		if !reflect.DeepEqual(pkgCfg.IgnoreFields, expectedIgnores) {
			t.Errorf("Expected ignored fields %v, got %v", expectedIgnores, pkgCfg.IgnoreFields)
		}

		// TypeConversionRules should be empty for p01_basic
		if len(pkgCfg.TypeConversionRules) != 0 {
			t.Errorf("Expected 0 TypeConversionRules, got %d", len(pkgCfg.TypeConversionRules))
		}
	})

	t.Run("LocalTypeNameToFQN", func(t *testing.T) {
		// This test is for the local aliases defined in directives.go
		nameToFQN := walker.GetLocalTypeNameToFQN()
		expected := map[string]string{
			"User":   "github.com/origadmin/abgen/testdata/fixture/ent.User",
			"UserPB": "github.com/origadmin/abgen/testdata/fixture/types.User",
		}

		if len(nameToFQN) != len(expected) {
			t.Errorf("Expected %d defined types, but got %d. Got: %v", len(expected), len(nameToFQN), nameToFQN)
		}

		for name, expectedFQN := range expected {
			if fqn, ok := nameToFQN[name]; !ok {
				t.Errorf("Expected local type %q to be defined, but it was not found.", name)
			} else if fqn != expectedFQN {
				t.Errorf("For local type %q, expected FQN %q, but got %q", name, expectedFQN, fqn)
			}
		}
	})
}
