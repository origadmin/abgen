package analyzer

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestComplexTypeResolution performs deep validation of the TypeInfo structure
// for various complex type definitions and aliases.
func TestComplexTypeResolution(t *testing.T) {
	// --- Setup ---
	// With the new design, we don't need to pre-load anything.
	// The walker will dynamically load packages as needed.
	walker := NewPackageWalker()

	// --- Test Cases ---
	tests := []struct {
		name         string
		fqn          string // Fully Qualified Name
		isAlias      bool
		expectedKind TypeKind
		expectedType string // The expected string representation of the type
		validate     func(t *testing.T, ti *TypeInfo)
	}{
		// --- Basic Struct ---
		{
			name:         "User",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.User",
			isAlias:      false,
			expectedKind: Struct,
			expectedType: "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.User",
			validate: func(t *testing.T, ti *TypeInfo) {
				if len(ti.Fields) != 5 {
					t.Errorf("Expected User to have 5 fields, got %d", len(ti.Fields))
				}
			},
		},
		// --- Alias to External Struct ---
		{
			name:         "UserAlias",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.UserAlias",
			isAlias:      true,
			expectedKind: Struct, // The alias resolves to a struct
			expectedType: "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.UserAlias",
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying == nil {
					t.Fatal("Underlying is nil for UserAlias")
				}
				if ti.Name != "UserAlias" {
					t.Errorf("Expected name to be UserAlias, got %s", ti.Name)
				}
				// The underlying type should be the resolved external.User
				if ti.Underlying.Name != "User" {
					t.Errorf("Expected Underlying.Name to be 'User', got '%s'", ti.Underlying.Name)
				}
				if ti.Underlying.ImportPath != "github.com/origadmin/abgen/testdata/00_complex_type_parsing/external" {
					t.Errorf("Expected Underlying.ImportPath to be external, got '%s'", ti.Underlying.ImportPath)
				}
				if ti.Underlying.Kind != Struct {
					t.Errorf("Expected Underlying.Kind to be Struct, got %v", ti.Underlying.Kind)
				}
				// The fields should be populated from the underlying struct
				if len(ti.Underlying.Fields) != 6 {
					t.Errorf("Expected Underlying to have 6 fields (from external.User), got %d", len(ti.Underlying.Fields))
				}
			},
		},
		// --- Defined Pointer to Struct ---
		{
			name:         "DefinedPtr",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.DefinedPtr",
			isAlias:      false,
			expectedKind: Pointer, // It's a named type, but its underlying kind is a pointer
			expectedType: "*github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying == nil {
					t.Fatal("Underlying is nil for DefinedPtr")
				}
				if ti.Underlying.Kind != Pointer {
					t.Fatalf("Expected Underlying kind to be Pointer, got %v", ti.Underlying.Kind)
				}
				// The element of the pointer
				elem := ti.Underlying.Underlying
				if elem == nil {
					t.Fatal("Pointer element is nil for DefinedPtr")
				}
				if elem.Name != "BaseStruct" {
					t.Errorf("Expected pointer element to be BaseStruct, got %s", elem.Name)
				}
				if elem.Kind != Struct {
					t.Errorf("Expected pointer element kind to be Struct, got %v", elem.Kind)
				}
				if len(elem.Fields) != 2 {
					t.Errorf("Expected BaseStruct to have 2 fields, got %d", len(elem.Fields))
				}
			},
		},
		// --- Alias to Pointer ---
		{
			name:         "AliasPtr",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.AliasPtr",
			isAlias:      true,
			expectedKind: Pointer, // It's an alias to a pointer
			expectedType: "*github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying == nil {
					t.Fatal("Underlying is nil for AliasPtr")
				}
				if ti.Underlying.Kind != Pointer {
					t.Errorf("Expected Underlying kind to be Pointer, got %v", ti.Underlying.Kind)
				}
				elem := ti.Underlying.Underlying
				if elem == nil {
					t.Fatal("Pointer element is nil for AliasPtr")
				}
				if elem.Name != "BaseStruct" {
					t.Errorf("Expected pointer element to be BaseStruct, got %s", elem.Name)
				}
			},
		},
		// --- Defined Slice of Pointers ---
		{
			name:         "DefinedSliceOfPtrs",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.DefinedSliceOfPtrs",
			isAlias:      false,
			expectedKind: Slice,
			expectedType: "[]*github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
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
		// --- Alias to Slice of Pointers ---
		{
			name:         "AliasSliceOfPtrs",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.AliasSliceOfPtrs",
			isAlias:      true,
			expectedKind: Slice,
			expectedType: "[]*github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
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
		// --- Defined Pointer to Slice ---
		{
			name:         "DefinedPtrToSlice",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.DefinedPtrToSlice",
			isAlias:      false,
			expectedKind: Pointer,
			expectedType: "*[]github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
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
		// --- Alias to Pointer to Slice ---
		{
			name:         "AliasPtrToSlice",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.AliasPtrToSlice",
			isAlias:      true,
			expectedKind: Pointer,
			expectedType: "*[]github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
		},
		// --- Defined Slice of Slices ---
		{
			name:         "DefinedSliceOfSlices",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.DefinedSliceOfSlices",
			isAlias:      false,
			expectedKind: Slice,
			expectedType: "[][]github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
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
		// --- Alias to Slice of Slices ---
		{
			name:         "AliasSliceOfSlices",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.AliasSliceOfSlices",
			isAlias:      true,
			expectedKind: Slice,
			expectedType: "[][]github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
		},
		// --- Defined Map ---
		{
			name:         "DefinedMap",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.DefinedMap",
			isAlias:      false,
			expectedKind: Map,
			expectedType: "map[string]*github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
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
		// --- Alias to Map ---
		{
			name:         "AliasMap",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.AliasMap",
			isAlias:      true,
			expectedKind: Map,
			expectedType: "map[string]*github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
		},
		// --- Defined Triple Pointer ---
		{
			name:         "TriplePtr",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.TriplePtr",
			isAlias:      false,
			expectedKind: Pointer,
			expectedType: "***github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.BaseStruct",
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying == nil {
					t.Fatal("Underlying is nil for TriplePtr (first pointer)")
				}
				if ti.Underlying.Kind != Pointer {
					t.Fatalf("Expected first Underlying kind to be Pointer, got %v", ti.Underlying.Kind)
				}

				secondPtr := ti.Underlying.Underlying
				if secondPtr == nil {
					t.Fatal("Underlying.Underlying is nil for TriplePtr (second pointer)")
				}
				if secondPtr.Kind != Pointer {
					t.Fatalf("Expected second Underlying kind to be Pointer, got %v", secondPtr.Kind)
				}

				thirdPtr := secondPtr.Underlying
				if thirdPtr == nil {
					t.Fatal("Underlying.Underlying.Underlying is nil for TriplePtr (third pointer)")
				}
				if thirdPtr.Kind != Pointer {
					t.Fatalf("Expected third Underlying kind to be Pointer, got %v", thirdPtr.Kind)
				}

				baseStruct := thirdPtr.Underlying
				if baseStruct == nil {
					t.Fatal("BaseStruct is nil for TriplePtr")
				}
				if baseStruct.Name != "BaseStruct" {
					t.Errorf("Expected final element to be BaseStruct, got %s", baseStruct.Name)
				}
				if baseStruct.Kind != Struct {
					t.Errorf("Expected final element kind to be Struct, got %v", baseStruct.Kind)
				}
			},
		},
		// --- Defined MyPtr (Pointer to int) ---
		{
			name:         "MyPtr",
			fqn:          "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.MyPtr",
			isAlias:      false,
			expectedKind: Pointer,
			expectedType: "*int",
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying == nil {
					t.Fatal("Underlying is nil for MyPtr")
				}
				if ti.Underlying.Kind != Pointer {
					t.Fatalf("Expected Underlying kind to be Pointer, got %v", ti.Underlying.Kind)
				}
				elem := ti.Underlying.Underlying
				if elem == nil {
					t.Fatal("Pointer element is nil for MyPtr")
				}
				if elem.Name != "int" {
					t.Errorf("Expected pointer element to be int, got %s", elem.Name)
				}
				if elem.Kind != Primitive {
					t.Errorf("Expected pointer element kind to be Primitive, got %v", elem.Kind)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti, err := walker.FindTypeByFQN(tt.fqn)
			if err != nil {
				// Provide more context on failure
				if len(walker.pkgs) > 0 {
					t.Log("Loaded packages:")
					for _, pkg := range walker.pkgs {
						t.Logf("- %s (%s)", pkg.Name, pkg.PkgPath)
					}
				}
				if walker.failedLoads != nil && len(walker.failedLoads) > 0 {
					t.Log("Failed loads:")
					for path := range walker.failedLoads {
						t.Logf("- %s", path)
					}
				}
				t.Fatalf("FindTypeByFQN failed for %s: %v", tt.name, err)
			}

			if ti.Name != tt.name {
				t.Errorf("Expected Name to be %s, got %s", tt.name, ti.Name)
			}
			if ti.IsAlias != tt.isAlias {
				t.Errorf("Expected IsAlias to be %v, got %v", tt.isAlias, ti.IsAlias)
			}

			// For aliases, the Kind is derived from the underlying type.
			// For defined types, the Kind is also from the underlying structure.
			if ti.Kind != tt.expectedKind {
				t.Errorf("Expected Kind to be %v, got %v", tt.expectedKind, ti.Kind)
			}

			// Validate reconstructed type string
			reconstructedType := ti.ToTypeString()
			if reconstructedType != tt.expectedType {
				t.Errorf("ToTypeString mismatch for %s:\nExpected: %s\nGot:      %s", tt.name, tt.expectedType, reconstructedType)
			}

			// Run specific deep validation if provided
			if tt.validate != nil {
				tt.validate(t, ti)
			}
		})
	}
}

// Helper to load packages for tests if needed, though the new design avoids this.
func loadTestPackages(t *testing.T, patterns ...string) []*packages.Package {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Tests:      false,
		BuildFlags: []string{"-tags=abgen"},
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		t.Fatalf("failed to load packages: %v", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatalf("packages contain errors")
	}
	return pkgs
}
