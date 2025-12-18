package analyzer

import (
	"log/slog"
	"os"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/model"
)

const (
	testPackagePath     = "github.com/origadmin/abgen/testdata/01_type_analysis/complex_type_parsing/all_complex_types"
	externalPackagePath = "github.com/origadmin/abgen/testdata/01_type_analysis/complex_type_parsing/external"
)

func TestMain(m *testing.M) {
	logHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(logHandler))
	slog.Info("TestMain", "testPackagePath", testPackagePath, "externalPackagePath", externalPackagePath)
	os.Exit(m.Run())
}

// loadTestPackages is a helper function to load a complete package graph for testing.
func loadTestPackages(t *testing.T, patterns ...string) []*packages.Package {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Tests:      false,
		BuildFlags: []string{"-tags=abgen"},
	}

	// Also load external package to ensure it's available
	allPatterns := append(patterns, externalPackagePath)

	pkgs, err := packages.Load(cfg, allPatterns...)
	if err != nil {
		t.Fatalf("failed to load packages: %v", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatalf("packages contain errors")
	}
	return pkgs
}

// TestTypeAnalyzer performs comprehensive validation of the TypeInfo structure.
func TestTypeAnalyzer(t *testing.T) {
	// --- Setup ---
	pkgs := loadTestPackages(t, testPackagePath)
	analyzer := NewTypeAnalyzer()
	analyzer.SetPackages(pkgs) // Manually set packages for this test

	// --- Test Cases ---
	type testCase struct {
		name                  string
		fqn                   string
		isAlias               bool
		expectedKind          model.TypeKind
		expectedUnderlyingFQN string
		validate              func(t *testing.T, ti *model.TypeInfo)
	}

	tests := []testCase{
		// === Basic Struct Types ===
		{
			name:         "User",
			fqn:          testPackagePath + ".User",
			isAlias:      false,
			expectedKind: model.Struct, // Correct: Named structs are 'Struct' kind.
			validate: func(t *testing.T, ti *model.TypeInfo) {
				expectedFields := []struct{ name, typeName string }{
					{"ID", "int64"},
					{"Name", "string"},
					{"Email", "string"},
					{"CreatedAt", "time.Time"},
					{"Status", "external.Status"},
				}
				if len(ti.Fields) != len(expectedFields) {
					t.Fatalf("Expected User to have %d fields, got %d", len(expectedFields), len(ti.Fields))
				}
				validateFields(t, ti.Fields, expectedFields)
			},
		},
		{
			name:         "Product",
			fqn:          testPackagePath + ".Product",
			isAlias:      false,
			expectedKind: model.Struct, // Correct: Named structs are 'Struct' kind.
			validate: func(t *testing.T, ti *model.TypeInfo) {
				expectedFields := []struct{ name, typeName string }{
					{"ProductID", "string"},
					{"Name", "string"},
					{"Price", "float64"},
				}
				if len(ti.Fields) != len(expectedFields) {
					t.Fatalf("Expected Product to have %d fields, got %d", len(expectedFields), len(ti.Fields))
				}
				validateFields(t, ti.Fields, expectedFields)
			},
		},

		// === Type Aliases (type T = U) ===
		{
			name:                  "UserAlias",
			fqn:                   testPackagePath + ".UserAlias",
			isAlias:               true,
			expectedKind:          model.Named,
			expectedUnderlyingFQN: externalPackagePath + ".User",
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil {
					t.Fatal("Underlying is nil for UserAlias")
				}
				// Correct: The underlying type is a named struct, so its Kind is Struct.
				if ti.Underlying.Kind != model.Struct {
					t.Errorf("Expected Underlying Kind to be Struct, got %v", ti.Underlying.Kind)
				}
				expectedFields := []struct{ name, typeName string }{
					{"ID", "int64"},
					{"FirstName", "string"},
					{"LastName", "string"},
					{"Email", "string"},
					{"CreatedAt", "time.Time"},
					{"Status", "external.Status"},
				}
				if len(ti.Underlying.Fields) != len(expectedFields) {
					t.Fatalf("Expected UserAlias's underlying struct to have %d fields, got %d", len(expectedFields), len(ti.Underlying.Fields))
				}
				validateFields(t, ti.Underlying.Fields, expectedFields)
			},
		},

		// === Defined Types (type T U) ===
		{
			name:         "DefinedPtr",
			fqn:          testPackagePath + ".DefinedPtr",
			isAlias:      false,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil || ti.Underlying.Kind != model.Pointer {
					t.Fatalf("Expected underlying to be Pointer, got %v", ti.Underlying)
				}
				if ti.Underlying.Underlying == nil || ti.Underlying.Underlying.Kind != model.Struct || ti.Underlying.Underlying.Name != "BaseStruct" {
					t.Fatalf("Expected underlying's underlying type to be struct BaseStruct, got '%v'", ti.Underlying.Underlying)
				}
			},
		},
		{
			name:         "DefinedSliceOfPtrs",
			fqn:          testPackagePath + ".DefinedSliceOfPtrs",
			isAlias:      false,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil || ti.Underlying.Kind != model.Slice {
					t.Fatalf("Expected underlying to be Slice, got %v", ti.Underlying)
				}
				if ti.Underlying.Underlying == nil || ti.Underlying.Underlying.Kind != model.Pointer {
					t.Errorf("Expected slice element to be Pointer, got %v", ti.Underlying.Underlying)
				}
			},
		},
		{
			name:         "DefinedPtrToSlice",
			fqn:          testPackagePath + ".DefinedPtrToSlice",
			isAlias:      false,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil || ti.Underlying.Kind != model.Pointer {
					t.Fatalf("Expected underlying to be Pointer, got %v", ti.Underlying)
				}
				if ti.Underlying.Underlying == nil || ti.Underlying.Underlying.Kind != model.Slice {
					t.Errorf("Expected pointer's element to be Slice, got %v", ti.Underlying.Underlying)
				}
			},
		},
		{
			name:         "DefinedSliceOfSlices",
			fqn:          testPackagePath + ".DefinedSliceOfSlices",
			isAlias:      false,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil || ti.Underlying.Kind != model.Slice {
					t.Fatalf("Expected underlying to be Slice, got %v", ti.Underlying)
				}
			},
		},
		{
			name:         "DefinedMap",
			fqn:          testPackagePath + ".DefinedMap",
			isAlias:      false,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil || ti.Underlying.Kind != model.Map {
					t.Fatalf("Expected underlying to be Map, got %v", ti.Underlying)
				}
				if ti.Underlying.KeyType == nil || ti.Underlying.KeyType.Kind != model.Primitive || ti.Underlying.KeyType.Name != "string" {
					t.Errorf("Expected key type to be string, got %v", ti.Underlying.KeyType)
				}
				if ti.Underlying.Underlying == nil || ti.Underlying.Underlying.Kind != model.Pointer {
					t.Errorf("Expected value type to be Pointer, got %v", ti.Underlying.Underlying)
				}
			},
		},
		{
			name:         "TriplePtr",
			fqn:          testPackagePath + ".TriplePtr",
			isAlias:      false,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil {
					t.Fatal("Underlying is nil for TriplePtr")
				}
				depth := countPointerDepth(ti.Underlying)
				if depth != 3 {
					t.Errorf("Expected pointer depth of 3 in underlying type, got %d", depth)
				}
			},
		},
		{
			name:         "MyPtr",
			fqn:          testPackagePath + ".MyPtr",
			isAlias:      false,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil || ti.Underlying.Kind != model.Pointer {
					t.Fatalf("Expected underlying to be Pointer, got %v", ti.Underlying)
				}
				if ti.Underlying.Underlying == nil || ti.Underlying.Underlying.Kind != model.Primitive || ti.Underlying.Underlying.Name != "int" {
					t.Errorf("Expected underlying's underlying to be int, got %v", ti.Underlying.Underlying)
				}
			},
		},

		// === Alias Types (type T = U) - Corrected Tests ===
		{
			name:         "AliasPtr",
			fqn:          testPackagePath + ".AliasPtr",
			isAlias:      true,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil || ti.Underlying.Kind != model.Pointer {
					t.Fatalf("Expected underlying to be Pointer, got %v", ti.Underlying)
				}
				if ti.Underlying.Underlying == nil || ti.Underlying.Underlying.Kind != model.Struct || ti.Underlying.Underlying.Name != "BaseStruct" {
					t.Fatalf("Expected underlying's underlying type to be struct BaseStruct, got %v", ti.Underlying.Underlying)
				}
			},
		},
		{
			name:         "AliasSliceOfPtrs",
			fqn:          testPackagePath + ".AliasSliceOfPtrs",
			isAlias:      true,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil || ti.Underlying.Kind != model.Slice {
					t.Fatalf("Expected underlying to be Slice, got %v", ti.Underlying)
				}
				if ti.Underlying.Underlying == nil || ti.Underlying.Underlying.Kind != model.Pointer {
					t.Errorf("Expected slice element to be Pointer, got %v", ti.Underlying.Underlying)
				}
				if ti.Underlying.Underlying.Underlying == nil || ti.Underlying.Underlying.Underlying.Kind != model.Struct || ti.Underlying.Underlying.Underlying.Name != "BaseStruct" {
					t.Errorf("Expected pointer target to be struct BaseStruct, got %v", ti.Underlying.Underlying.Underlying)
				}
			},
		},
		{
			name:         "AliasMap",
			fqn:          testPackagePath + ".AliasMap",
			isAlias:      true,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Underlying == nil || ti.Underlying.Kind != model.Map {
					t.Fatalf("Expected underlying to be Map, got %v", ti.Underlying)
				}
				if ti.Underlying.KeyType == nil || ti.Underlying.KeyType.Kind != model.Primitive || ti.Underlying.KeyType.Name != "string" {
					t.Errorf("Expected key type to be string, got %v", ti.Underlying.KeyType)
				}
				if ti.Underlying.Underlying == nil || ti.Underlying.Underlying.Kind != model.Pointer {
					t.Errorf("Expected map value type to be Pointer, got %v", ti.Underlying.Underlying)
				}
				if ti.Underlying.Underlying.Underlying == nil || ti.Underlying.Underlying.Underlying.Kind != model.Struct || ti.Underlying.Underlying.Underlying.Name != "BaseStruct" {
					t.Errorf("Expected pointer target to be struct BaseStruct, got %v", ti.Underlying.Underlying.Underlying)
				}
			},
		},

		// === External Types ===
		{
			name:         "User",
			fqn:          externalPackagePath + ".User",
			isAlias:      false,
			expectedKind: model.Struct,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				expectedFields := []struct{ name, typeName string }{
					{"ID", "int64"},
					{"FirstName", "string"},
					{"LastName", "string"},
					{"Email", "string"},
					{"CreatedAt", "time.Time"},
					{"Status", "external.Status"},
				}
				if len(ti.Fields) != len(expectedFields) {
					t.Fatalf("Expected external.User to have %d fields, got %d", len(expectedFields), len(ti.Fields))
				}
				validateFields(t, ti.Fields, expectedFields)
			},
		},
		{
			name:         "Status",
			fqn:          externalPackagePath + ".Status",
			isAlias:      false,
			expectedKind: model.Named,
			validate: func(t *testing.T, ti *model.TypeInfo) {
				if ti.Name != "Status" {
					t.Errorf("Expected name to be Status, got %s", ti.Name)
				}
				if ti.Underlying == nil || ti.Underlying.Kind != model.Primitive || ti.Underlying.Name != "int32" {
					t.Errorf("Expected underlying to be int32 Primitive, got %v", ti.Underlying)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti, err := analyzer.Find(tt.fqn)
			if err != nil {
				t.Fatalf("Find failed for %s: %v", tt.name, err)
			}

			// Basic validation
			if ti.Name != tt.name {
				t.Errorf("Expected Name to be %s, got %s", tt.name, ti.Name)
			}
			if ti.IsAlias != tt.isAlias {
				t.Errorf("Expected IsAlias to be %v, got %v", tt.isAlias, ti.IsAlias)
			}
			if ti.Kind != tt.expectedKind {
				t.Errorf("Expected Kind to be %v, got %v", tt.expectedKind, ti.Kind)
			}

			// Underlying type validation
			if tt.expectedUnderlyingFQN != "" {
				if ti.Underlying == nil {
					t.Fatalf("Expected Underlying FQN %s but Underlying is nil", tt.expectedUnderlyingFQN)
				}
				if ti.Underlying.FQN() != tt.expectedUnderlyingFQN {
					t.Errorf("Expected Underlying FQN to be %s, got %s", tt.expectedUnderlyingFQN, ti.Underlying.FQN())
				}
			}

			// Custom validation
			if tt.validate != nil {
				tt.validate(t, ti)
			}
		})
	}
}

// validateFields validates field names and types
func validateFields(t *testing.T, fields []*model.FieldInfo, expectedFields []struct{ name, typeName string }) {
	for i, expected := range expectedFields {
		if i >= len(fields) {
			t.Errorf("Expected field %d '%s' but only have %d fields", i, expected.name, len(fields))
			continue
		}

		field := fields[i]
		if field.Name != expected.name {
			t.Errorf("Expected field %d name to be '%s', got '%s'", i, expected.name, field.Name)
		}

		if field.Type != nil {
			typeName := field.Type.TypeString()
			if typeName != expected.typeName {
				t.Errorf("Expected field %d '%s' type to be '%s', got '%s'", i, field.Name, expected.typeName, typeName)
			}
		} else {
			t.Errorf("Field %d '%s' has nil type", i, field.Name)
		}
	}
}

// countPointerDepth counts the nested pointer depth
func countPointerDepth(ti *model.TypeInfo) int {
	depth := 0
	current := ti
	for current != nil && current.Kind == model.Pointer {
		depth++
		current = current.Underlying
	}
	return depth
}
