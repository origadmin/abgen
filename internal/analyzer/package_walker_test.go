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
		name                 string
		fqn                  string // Fully Qualified Name
		isAlias              bool
		expectedKind         TypeKind
		expectedGoTypeString string // The expected string representation of the type from GoTypeString()
		validate             func(t *testing.T, ti *TypeInfo)
	}{
		// --- Basic Struct ---
		{
			name:                 "User",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.User",
			isAlias:              false,
			expectedKind:         Struct,
			expectedGoTypeString: "User", // Changed from FQN
			validate: func(t *testing.T, ti *TypeInfo) {
				if len(ti.Fields) != 5 {
					t.Errorf("Expected User to have 5 fields, got %d", len(ti.Fields))
				}
			},
		},
		// --- Alias to External Struct ---
		{
			name:                 "UserAlias",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.UserAlias",
			isAlias:              true,
			expectedKind:         Struct,      // The alias resolves to a struct
			expectedGoTypeString: "UserAlias", 
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying == nil {
					t.Fatal("Underlying is nil for UserAlias")
				}
				if ti.Name != "UserAlias" {
					t.Errorf("Expected name to be UserAlias, got %s", ti.Name)
				}
				// Because types.Alias.Underlying() returns the raw struct, not the types.Named wrapper for external.User,
				// the Underlying TypeInfo will represent the anonymous struct directly.
				if ti.Underlying.Name != "" { // Expected to be empty for the anonymous struct
					t.Errorf("Expected Underlying.Name to be '', got '%s'", ti.Underlying.Name)
				}
				if ti.Underlying.ImportPath != "" { // Expected to be empty for the anonymous struct
					t.Errorf("Expected Underlying.ImportPath to be '', got '%s'", ti.Underlying.ImportPath)
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
			name:                 "DefinedPtr",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.DefinedPtr",
			isAlias:              false,
			expectedKind:         Pointer,       // It's a named type, but its underlying kind is a pointer
			expectedGoTypeString: "*BaseStruct", 
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying == nil { // ti.Underlying is TypeInfo(BaseStruct)
					t.Fatal("Underlying is nil for DefinedPtr")
				}
				if ti.Underlying.Name != "BaseStruct" { // Checks TypeInfo(BaseStruct).Name
					t.Errorf("Expected Underlying name to be BaseStruct, got %s", ti.Underlying.Name)
				}
				if ti.Underlying.Kind != Struct { // Checks TypeInfo(BaseStruct).Kind
					t.Errorf("Expected Underlying kind to be Struct, got %v", ti.Underlying.Kind)
				}
				if len(ti.Underlying.Fields) != 2 { // Fields are from BaseStruct
					t.Errorf("Expected BaseStruct to have 2 fields, got %d", len(ti.Underlying.Fields))
				}
			},
		},
		// --- Alias to Pointer ---
		{
			name:                 "AliasPtr",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.AliasPtr",
			isAlias:              true,
			expectedKind:         Pointer,       // It's an alias to a pointer
			expectedGoTypeString: "*BaseStruct", 
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.Underlying == nil { // ti.Underlying is TypeInfo(BaseStruct)
					t.Fatal("Underlying is nil for AliasPtr")
				}
				if ti.Underlying.Name != "BaseStruct" { // Checks TypeInfo(BaseStruct).Name
					t.Errorf("Expected Underlying name to be BaseStruct, got %s", ti.Underlying.Name)
				}
				if ti.Underlying.Kind != Struct { // Checks TypeInfo(BaseStruct).Kind
					t.Errorf("Expected Underlying kind to be Struct, got %v", ti.Underlying.Kind)
				}
			},
		},
		// --- Defined Slice of Pointers ---
		{
			name:                 "DefinedSliceOfPtrs",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.DefinedSliceOfPtrs",
			isAlias:              false,
			expectedKind:         Slice,
			expectedGoTypeString: "[]*BaseStruct", 
			validate: func(t *testing.T, ti *TypeInfo) {
				ptr := ti.Underlying // This is TypeInfo(*BaseStruct)
				if ptr.Kind != Pointer {
					t.Fatalf("Expected Underlying kind to be Pointer, got %v", ptr.Kind)
				}
				strct := ptr.Underlying // This is TypeInfo(BaseStruct)
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
			name:                 "AliasSliceOfPtrs",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.AliasSliceOfPtrs",
			isAlias:              true,
			expectedKind:         Slice,
			expectedGoTypeString: "[]*BaseStruct", 
			validate: func(t *testing.T, ti *TypeInfo) {
				ptr := ti.Underlying // This is TypeInfo(*BaseStruct)
				if ptr.Kind != Pointer {
					t.Fatalf("Expected Underlying kind to be Pointer, got %v", ptr.Kind)
				}
				strct := ptr.Underlying // This is TypeInfo(BaseStruct)
				if strct.Kind != Struct {
					t.Fatalf("Expected pointer element kind to be Struct, got %v", strct.Kind)
				}
				if strct.Name != "BaseStruct" {
					t.Errorf("Expected final element to be BaseStruct, got %s", strct.Name)
				}
			},
		},
		// --- Defined Pointer to Slice ---
		{
			name:                 "DefinedPtrToSlice",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.DefinedPtrToSlice",
			isAlias:              false,
			expectedKind:         Pointer,
			expectedGoTypeString: "*[]BaseStruct", 
			validate: func(t *testing.T, ti *TypeInfo) {
				slice := ti.Underlying // This is TypeInfo([]BaseStruct)
				if slice.Kind != Slice {
					t.Fatalf("Expected Underlying kind to be Slice, got %v", slice.Kind)
				}
				strct := slice.Underlying // This is TypeInfo(BaseStruct)
				if strct.Kind != Struct {
					t.Fatalf("Expected slice element kind to be Struct, got %v", strct.Kind)
				}
			},
		},
		// --- Alias to Pointer to Slice ---
		{
			name:                 "AliasPtrToSlice",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.AliasPtrToSlice",
			isAlias:              true,
			expectedKind:         Pointer,
			expectedGoTypeString: "*[]BaseStruct", // Changed from FQN
		},
		// --- Defined Slice of Slices ---
		{
			name:                 "DefinedSliceOfSlices",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.DefinedSliceOfSlices",
			isAlias:              false,
			expectedKind:         Slice,
			expectedGoTypeString: "[][]BaseStruct", 
			validate: func(t *testing.T, ti *TypeInfo) {
				innerSlice := ti.Underlying // This is TypeInfo([]BaseStruct)
				if innerSlice.Kind != Slice {
					t.Fatalf("Expected Underlying to be Slice, got %v", innerSlice.Kind)
				}
				strct := innerSlice.Underlying // This is TypeInfo(BaseStruct)
				if strct.Kind != Struct {
					t.Fatalf("Expected final element to be Struct, got %v", strct.Kind)
				}
			},
		},
		// --- Alias to Slice of Slices ---
		{
			name:                 "AliasSliceOfSlices",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.AliasSliceOfSlices",
			isAlias:              true,
			expectedKind:         Slice,
			expectedGoTypeString: "[][]BaseStruct", // Changed from FQN
		},
		// --- Defined Map ---
		{
			name:                 "DefinedMap",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.DefinedMap",
			isAlias:              false,
			expectedKind:         Map,
			expectedGoTypeString: "map[string]*BaseStruct", 
			validate: func(t *testing.T, ti *TypeInfo) {
				if ti.KeyType.Kind != Primitive || ti.KeyType.Name != "string" {
					t.Errorf("Expected map key to be string, got %v", ti.KeyType)
				}
				valType := ti.Underlying // This is TypeInfo(*BaseStruct)
				if valType.Kind != Pointer || valType.Underlying.Name != "BaseStruct" {
					t.Errorf("Expected map value to be *BaseStruct, got %v", valType)
				}
			},
		},
		// --- Alias to Map ---
		{
			name:                 "AliasMap",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.AliasMap",
			isAlias:              true,
			expectedKind:         Map,
			expectedGoTypeString: "map[string]*BaseStruct", // Changed from FQN
		},
		// --- Defined Triple Pointer ---
		{
			name:                 "TriplePtr",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.TriplePtr",
			isAlias:              false,
			expectedKind:         Pointer,
			expectedGoTypeString: "***BaseStruct", 
			validate: func(t *testing.T, ti *TypeInfo) {
				firstPtr := ti.Underlying // This is TypeInfo(**BaseStruct)
				if firstPtr.Kind != Pointer {
					t.Fatalf("Expected first Underlying kind to be Pointer, got %v", firstPtr.Kind)
				}

				secondPtr := firstPtr.Underlying // This is TypeInfo(*BaseStruct)
				if secondPtr.Kind != Pointer {
					t.Fatalf("Expected second Underlying kind to be Pointer, got %v", secondPtr.Kind)
				}

				baseStruct := secondPtr.Underlying // This is TypeInfo(BaseStruct)
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
			name:                 "MyPtr",
			fqn:                  "github.com/origadmin/abgen/testdata/00_complex_type_parsing/all_complex_types.MyPtr",
			isAlias:              false,
			expectedKind:         Pointer,
			expectedGoTypeString: "*int", 
			validate: func(t *testing.T, ti *TypeInfo) {
				elem := ti.Underlying // This is TypeInfo(int)
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
			reconstructedType := ti.GoTypeString() // Changed from ToTypeString()
			if reconstructedType != tt.expectedGoTypeString {
				t.Errorf("GoTypeString mismatch for %s:\nExpected: %s\nGot:      %s", tt.name, tt.expectedGoTypeString, reconstructedType)
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
