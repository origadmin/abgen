package ast

import (
	"path/filepath"
	"reflect"
	"sort"
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

// TestWalker_P01_EndToEnd performs an end-to-end integration test for the p01_basic use case.
// It verifies that after a full walk, the final PackageConversionConfig is correctly generated.
func TestWalker_P01_EndToEnd(t *testing.T) {
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

	err := walker.Analyze(directivePkg) // Analyze the directive package to extract configs
	if err != nil {
		t.Fatalf("Walker.Analyze() failed: %v", err)
	}

	t.Run("VerifyFinalPackageConfig", func(t *testing.T) {
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

}

// TestWalker_P01_AnalysisPhase specifically tests the analysis phase of the walker.
// It verifies that after processing the directive file, the internal state, such as
// the mapping of local type aliases to their fully-qualified names (FQN), is correct.
// This is crucial for all subsequent rule applications.
func TestWalker_P01_AnalysisPhase(t *testing.T) {
	// Load packages required for the test.
	allPkgs, directivePkg := loadTestPackages(t,
		"../../testdata/directives/p01_basic",
		"github.com/origadmin/abgen/testdata/fixture/ent",
		"github.com/origadmin/abgen/testdata/fixture/types",
	)

	// Initialize the walker and perform the walk.
	graph := make(types.ConversionGraph)
	walker := NewPackageWalker(graph)
	walker.AddPackages(allPkgs...)
	if err := walker.Analyze(directivePkg); err != nil {
		t.Fatalf("Walker.Analyze() failed: %v", err)
	}

	// Define the expected mapping from local type aliases to their FQNs.
	expectedAliasMap := map[string]string{
		"User":   "github.com/origadmin/abgen/testdata/fixture/ent.User",
		"UserPB": "github.com/origadmin/abgen/testdata/fixture/types.User",
	}

	// Retrieve the actual map from the walker.
	actualAliasMap := walker.GetLocalTypeNameToFQN()

	// Assert that the actual map matches the expected one.
	t.Run("VerifyLocalAliasToFQNMapping", func(t *testing.T) {
		if !reflect.DeepEqual(expectedAliasMap, actualAliasMap) {
			t.Errorf("Mismatched local type alias to FQN map.\nExpected: %v\nGot:      %v", expectedAliasMap, actualAliasMap)
		}
	})
}

// TestWalker_P01_DiscoveryPhase specifically tests the package discovery phase.
// It provides the walker with only the directive package and verifies that it
// correctly identifies the source and target packages that need to be loaded.
func TestWalker_P01_DiscoveryPhase(t *testing.T) {
	// Step 1: Load ONLY the directive package, simulating the initial state.
	_, directivePkg := loadTestPackages(t,
		"../../testdata/directives/p01_basic",
		// NO other dependencies are provided here.
	)

	// Step 2: Initialize the walker and run the discovery phase.
	graph := make(types.ConversionGraph)
	walker := NewPackageWalker(graph)
	discoveredPaths, err := walker.DiscoverPackages(directivePkg)
	if err != nil {
		t.Fatalf("Walker.DiscoverPackages() failed: %v", err)
	}

	// Step 3: Verify the results.
	expectedPaths := []string{
		"github.com/origadmin/abgen/testdata/fixture/ent",
		"github.com/origadmin/abgen/testdata/fixture/types",
	}

	// Sort slices for consistent comparison
	sort.Strings(discoveredPaths)
	sort.Strings(expectedPaths)

	if !reflect.DeepEqual(expectedPaths, discoveredPaths) {
		t.Errorf("Discovered package paths do not match expected paths.\nExpected: %v\nGot:      %v", expectedPaths, discoveredPaths)
	}
}

// TestWalker_P01_ParsedTypeStructure verifies that after the analysis phase,
// the walker has correctly parsed the structure of the source and target types
// (e.g., ent.User and types.User) and stored them in its internal type cache.
// This is a critical check to ensure the generator has accurate information
// before applying rules and generating code.
func TestWalker_P01_ParsedTypeStructure(t *testing.T) {
	// Step 1: Load all necessary packages.
	allPkgs, directivePkg := loadTestPackages(t,
		"../../testdata/directives/p01_basic",
		"github.com/origadmin/abgen/testdata/fixture/ent",
		"github.com/origadmin/abgen/testdata/fixture/types",
	)

	// Step 2: Initialize the walker and run the analysis.
	graph := make(types.ConversionGraph)
	walker := NewPackageWalker(graph)
	walker.AddPackages(allPkgs...)
	if err := walker.Analyze(directivePkg); err != nil {
		t.Fatalf("Walker.Analyze() failed: %v", err)
	}

	// Step 3: Retrieve the type cache and verify its contents.
	typeCache := walker.GetTypeCache()

	// Define constants for FQNs to avoid magic strings.
	const (
		entUserFQN   = "github.com/origadmin/abgen/testdata/fixture/ent.User"
		typesUserFQN = "github.com/origadmin/abgen/testdata/fixture/types.User"
	)

	// --- Verification for ent.User ---
	t.Run("VerifyEntUserStructure", func(t *testing.T) {
		info, ok := typeCache[entUserFQN]
		if !ok {
			t.Fatalf("Expected type cache to contain info for %q, but it was not found.", entUserFQN)
		}

		// Verify basic info
		if info.Name != "User" || info.ImportPath != "github.com/origadmin/abgen/testdata/fixture/ent" {
			t.Errorf("Incorrect basic info for ent.User. Got Name: %q, ImportPath: %q", info.Name, info.ImportPath)
		}

		// Verify that at least some key fields are correctly parsed.
		// A full field-by-field comparison can be overly brittle.
		expectedFields := map[string]string{
			"ID":       "int",
			"Username": "string",
			"Password": "string",
		}
		actualFields := make(map[string]string)
		for _, f := range info.Fields {
			actualFields[f.Name] = f.Type
		}

		for name, typeStr := range expectedFields {
			if actualType, ok := actualFields[name]; !ok {
				t.Errorf("Expected field %q not found in parsed ent.User", name)
			} else if actualType != typeStr {
				t.Errorf("For field %q, expected type %q, got %q", name, typeStr, actualType)
			}
		}
	})

	// --- Verification for types.User ---
	t.Run("VerifyTypesUserStructure", func(t *testing.T) {
		info, ok := typeCache[typesUserFQN]
		if !ok {
			t.Fatalf("Expected type cache to contain info for %q, but it was not found.", typesUserFQN)
		}

		if info.Name != "User" || info.ImportPath != "github.com/origadmin/abgen/testdata/fixture/types" {
			t.Errorf("Incorrect basic info for types.User. Got Name: %q, ImportPath: %q", info.Name, info.ImportPath)
		}

		// A simple check for a key field is sufficient here.
		foundId := false
		for _, f := range info.Fields {
			if f.Name == "Id" && f.Type == "int" {
				foundId = true
				break
			}
		}
		if !foundId {
			t.Error("Expected to find field 'Id' of type 'int' in parsed types.User")
		}
	})
}
