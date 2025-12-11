package analyzer

import (
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/model"
)

const (
	testPackagePath     = "github.com/origadmin/abgen/testdata/01_type_analysis/01_complex_types/source"
	externalPackagePath = "github.com/origadmin/abgen/testdata/01_type_analysis/01_complex_types/external"
)

// loadTestPackages is a helper function to load a complete package graph for testing.
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

// TestTypeAnalyzer performs deep validation of the TypeInfo structure.
func TestTypeAnalyzer(t *testing.T) {
	// --- Setup ---
	pkgs := loadTestPackages(t, testPackagePath)
	analyzer := NewTypeAnalyzer()
	analyzer.pkgs = pkgs // Manually set packages for this test

	// --- Test Cases ---
	type testCase struct {
		name                 string
		fqn                  string
		isAlias              bool
		expectedKind         model.TypeKind
		expectedUnderlyingFQN string
		validate             func(t *testing.T, ti *model.TypeInfo)
	}

	tests := []testCase{
		{
			name:         "User",
			fqn:          testPackagePath + ".User",
			isAlias:      false,
			expectedKind: model.Struct,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if len(ti.Fields) != 5 {
					t.Errorf("Expected User to have 5 fields, got %d", len(ti.Fields))
				}
			},
		},
		{
			name:                 "UserAlias",
			fqn:                  testPackagePath + ".UserAlias",
			isAlias:              true,
			expectedKind:         model.Struct,
			expectedUnderlyingFQN: externalPackagePath + ".User",
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil {
					t.Fatal("Underlying is nil for UserAlias")
				}
				if ti.Underlying.FQN() != externalPackagePath + ".User" {
					t.Errorf("Expected Underlying FQN to be %s, got %s", externalPackagePath + ".User", ti.Underlying.FQN())
				}
				if len(ti.Fields) != 6 {
					t.Errorf("Expected UserAlias to expose 6 fields from external.User, got %d", len(ti.Fields))
				}
			},
		},
		{
			name:         "EmbeddedStruct",
			fqn:          testPackagePath + ".EmbeddedStruct",
			isAlias:      false,
			expectedKind: model.Struct,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				// Expecting 3 fields: ID, Name (from embedded BaseStruct) and Timestamp
				if len(ti.Fields) != 3 {
					t.Fatalf("Expected EmbeddedStruct to have 3 fields (ID, Name, Timestamp), got %d", len(ti.Fields))
				}

				fieldNames := make(map[string]bool)
				for _, f := range ti.Fields {
					fieldNames[f.Name] = true
				}

				if !fieldNames["ID"] {
					t.Error("Expected to find promoted field 'ID'")
				}
				if !fieldNames["Name"] {
					t.Error("Expected to find promoted field 'Name'")
				}
				if !fieldNames["Timestamp"] {
					t.Error("Expected to find field 'Timestamp'")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti, err := analyzer.find(tt.fqn)
			if err != nil {
				t.Fatalf("Find failed for %s: %v", tt.name, err)
			}

			if ti.Name != tt.name {
				t.Errorf("Expected Name to be %s, got %s", tt.name, ti.Name)
			}
			if ti.IsAlias != tt.isAlias {
				t.Errorf("Expected IsAlias to be %v, got %v", tt.isAlias, ti.IsAlias)
			}
			if ti.Kind != tt.expectedKind {
				t.Errorf("Expected Kind to be %v, got %v", tt.expectedKind, ti.Kind)
			}

			if tt.validate != nil {
				tt.validate(t, ti)
			}
		})
	}
}
