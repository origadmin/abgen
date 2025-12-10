package analyzer

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestStagedParsing validates the entire staged parsing process.
func TestStagedParsing(t *testing.T) {
	// ... (this test remains unchanged)
	walker := NewPackageWalker()
	absPath, err := filepath.Abs("../../testdata/01_dependency_resolving/pkg_a")
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}
	initialPkg, err := walker.LoadInitialPackage(absPath)
	if err != nil {
		t.Fatalf("LoadInitialPackage failed: %v", err)
	}
	if initialPkg.Name != "pkg_a" {
		t.Errorf("Expected initial package name to be 'pkg_a', got '%s'", initialPkg.Name)
	}
	directives := walker.DiscoverDirectives(initialPkg)
	if len(directives) != 3 {
		t.Fatalf("Expected to find 3 directives, but found %d", len(directives))
	}
	dependencies := walker.ExtractDependencies(directives)
	if len(dependencies) != 2 {
		t.Fatalf("Expected to extract 2 dependencies, but found %d", len(dependencies))
	}
	allPkgs, err := walker.LoadFullGraph(absPath, dependencies...)
	if err != nil {
		t.Fatalf("LoadFullGraph failed: %v", err)
	}
	loadedPaths := make(map[string]bool)
	for _, pkg := range allPkgs {
		loadedPaths[pkg.PkgPath] = true
	}
	// Note: The FQN in tests must match the module path from go.mod
	if !loadedPaths["github.com/origadmin/abgen/testdata/01_dependency_resolving/pkg_a"] {
		t.Error("Full graph does not contain pkg_a")
	}
	if !loadedPaths["github.com/origadmin/abgen/testdata/01_dependency_resolving/pkg_b"] {
		t.Error("Full graph does not contain pkg_b")
	}
	if !loadedPaths["github.com/origadmin/abgen/testdata/01_dependency_resolving/pkg_c"] {
		t.Error("Full graph does not contain pkg_c")
	}
}

// TestComplexTypeResolution performs deep validation of the TypeInfo structure
// for various complex type definitions and aliases.
func TestComplexTypeResolution(t *testing.T) {
	// --- Setup ---
	walker := NewPackageWalker()
	absPath, err := filepath.Abs("../../testdata/00_complex_type_parsing/all_complex_types")
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}
	_, err = walker.LoadFullGraph(absPath)
	if err != nil {
		t.Fatalf("LoadFullGraph for complex types failed: %v", err)
	}

	// --- Test Cases ---
	tests := []struct {
		name         string
		isAlias      bool
		expectedKind TypeKind // Kind of the Underlying type
		validate     func(t *testing.T, ti *TypeInfo)
	}{
		// --- Basic Type Tests ---
		{
			name:         "User",
			isAlias:      false,
			expectedKind: Struct,
			validate: func(t *testing.T, ti *TypeInfo) {
				if len(ti.Fields) != 5 {
					t.Errorf("Expected User to have 5 fields, got %d", len(ti.Fields))
				}
			},
		},
		{
			name:         "UserAlias",
			isAlias:      true,
			expectedKind: Struct,
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying.Name != "User" {
					t.Errorf("Expected UserAlias to be an alias of User, got %s", ti.Underlying.Name)
				}
			},
		},
		// --- Complex Type Tests ---
		{
			name:         "DefinedPtr",
			isAlias:      false,
			expectedKind: Pointer,
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying.Kind != Pointer {
					t.Errorf("Expected Underlying kind to be Pointer, got %v", ti.Underlying.Kind)
				}
				if ti.Underlying.Underlying.Kind != Struct {
					t.Errorf("Expected Underlying's Underlying kind to be Struct, got %v", ti.Underlying.Underlying.Kind)
				}
				if ti.Underlying.Underlying.Name != "BaseStruct" {
					t.Errorf("Expected final underlying type to be BaseStruct, got %s", ti.Underlying.Underlying.Name)
				}
			},
		},
		{
			name:         "AliasPtr",
			isAlias:      true,
			expectedKind: Pointer,
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying.Kind != Pointer {
					t.Errorf("Expected Underlying kind to be Pointer, got %v", ti.Underlying.Kind)
				}
				if ti.Underlying.Underlying.Kind != Struct {
					t.Errorf("Expected Underlying's Underlying kind to be Struct, got %v", ti.Underlying.Underlying.Kind)
				}
			},
		},
		{
			name:         "DefinedSliceOfPtrs",
			isAlias:      false,
			expectedKind: Slice,
			validate: func(t *testing.T, ti *TypeInfo) {
				slice := ti.Underlying
				if slice.Kind != Slice {
					t.Fatalf("Expected Underlying kind to be Slice, got %v", slice.Kind)
				}
				pointer := slice.Underlying
				if pointer.Kind != Pointer {
					t.Fatalf("Expected slice element kind to be Pointer, got %v", pointer.Kind)
				}
				strct := pointer.Underlying
				if strct.Kind != Struct {
					t.Fatalf("Expected pointer element kind to be Struct, got %v", strct.Kind)
				}
				if strct.Name != "BaseStruct" {
					t.Errorf("Expected final element to be BaseStruct, got %s", strct.Name)
				}
			},
		},
		{
			name:         "AliasSliceOfPtrs",
			isAlias:      true,
			expectedKind: Slice,
			validate: func(t *testing.T, ti *TypeInfo) {
				slice := ti.Underlying
				if slice.Kind != Slice {
					t.Fatalf("Expected Underlying kind to be Slice, got %v", slice.Kind)
				}
				pointer := slice.Underlying
				if pointer.Kind != Pointer {
					t.Fatalf("Expected slice element kind to be Pointer, got %v", pointer.Kind)
				}
			},
		},
		{
			name:         "DefinedPtrToSlice",
			isAlias:      false,
			expectedKind: Pointer,
			validate: func(t *testing.T, ti *TypeInfo) {
				pointer := ti.Underlying
				if pointer.Kind != Pointer {
					t.Fatalf("Expected Underlying kind to be Pointer, got %v", pointer.Kind)
				}
				slice := pointer.Underlying
				if slice.Kind != Slice {
					t.Fatalf("Expected pointer element kind to be Slice, got %v", slice.Kind)
				}
				strct := slice.Underlying
				if strct.Kind != Struct {
					t.Fatalf("Expected slice element kind to be Struct, got %v", strct.Kind)
				}
			},
		},
		{
			name:         "AliasPtrToSlice",
			isAlias:      true,
			expectedKind: Pointer,
		},
		{
			name:         "DefinedSliceOfSlices",
			isAlias:      false,
			expectedKind: Slice,
			validate: func(t *testing.T, ti *TypeInfo) {
				outerSlice := ti.Underlying
				if outerSlice.Kind != Slice {
					t.Fatalf("Expected Underlying to be Slice, got %v", outerSlice.Kind)
				}
				innerSlice := outerSlice.Underlying
				if innerSlice.Kind != Slice {
					t.Fatalf("Expected inner type to be Slice, got %v", innerSlice.Kind)
				}
				strct := innerSlice.Underlying
				if strct.Kind != Struct {
					t.Fatalf("Expected final element to be Struct, got %v", strct.Kind)
				}
			},
		},
		{
			name:         "AliasSliceOfSlices",
			isAlias:      true,
			expectedKind: Slice,
		},
		{
			name:         "DefinedMap",
			isAlias:      false,
			expectedKind: Map,
			validate: func(t *testing.T, ti *TypeInfo) {
				mp := ti.Underlying
				if mp.Kind != Map {
					t.Fatalf("Expected Underlying to be Map, got %v", mp.Kind)
				}
				if mp.KeyType.Kind != Primitive || mp.KeyType.Name != "string" {
					t.Errorf("Expected map key to be string, got %v", mp.KeyType)
				}
				if mp.Underlying.Kind != Pointer || mp.Underlying.Underlying.Name != "BaseStruct" {
					t.Errorf("Expected map value to be *BaseStruct, got %v", mp.Underlying)
				}
			},
		},
		{
			name:         "AliasMap",
			isAlias:      true,
			expectedKind: Map,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Corrected FQN to use the proper module path.
			fqn := "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types." + tt.name
			ti, err := walker.FindTypeByFQN(fqn)
			if err != nil {
				t.Fatalf("FindTypeByFQN failed for %s: %v", tt.name, err)
			}

			if ti.Name != tt.name {
				t.Errorf("Expected Name to be %s, got %s", tt.name, ti.Name)
			}
			if ti.IsAlias != tt.isAlias {
				t.Errorf("Expected IsAlias to be %v, got %v", tt.isAlias, ti.IsAlias)
			}
			if ti.Underlying == nil {
				t.Fatal("Underlying is nil")
			}
			if ti.Underlying.Kind != tt.expectedKind {
				t.Errorf("Expected Underlying.Kind to be %v, got %v", tt.expectedKind, ti.Underlying.Kind)
			}

			// Run specific deep validation if provided
			if tt.validate != nil {
				tt.validate(t, ti)
			}
		})
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if strings.Contains(v, str) {
			return true
		}
	}
	return false
}
