package model

import (
	"testing"
)

func TestTypeInfoMethods(t *testing.T) {
	// Test validation
	t.Run("IsValid", func(t *testing.T) {
		tests := []struct {
			name     string
			typeInfo *TypeInfo
			want     bool
		}{
			{
				name:     "nil type",
				typeInfo: nil,
				want:     false,
			},
			{
				name: "unknown type",
				typeInfo: &TypeInfo{
					Kind: Unknown,
				},
				want: false,
			},
			{
				name: "valid primitive",
				typeInfo: &TypeInfo{
					Kind: Primitive,
					Name: "int",
				},
				want: true,
			},
			{
				name: "valid array with underlying",
				typeInfo: &TypeInfo{
					Kind:     Array,
					ArrayLen: 5,
					Underlying: &TypeInfo{
						Kind: Primitive,
						Name: "int",
					},
				},
				want: true,
			},
			{
				name: "invalid array without underlying",
				typeInfo: &TypeInfo{
					Kind:     Array,
					ArrayLen: 5,
				},
				want: false,
			},
			{
				name: "valid map with key and underlying",
				typeInfo: &TypeInfo{
					Kind: Map,
					KeyType: &TypeInfo{
						Kind: Primitive,
						Name: "string",
					},
					Underlying: &TypeInfo{
						Kind: Primitive,
						Name: "int",
					},
				},
				want: true,
			},
			{
				name: "valid named type",
				typeInfo: &TypeInfo{
					Kind:       Named,
					Name:       "User",
					ImportPath: "github.com/example/pkg",
				},
				want: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.typeInfo.IsValid(); got != tt.want {
					t.Errorf("TypeInfo.IsValid() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	// Test string representations
	t.Run("String", func(t *testing.T) {
		tests := []struct {
			name     string
			typeInfo *TypeInfo
			want     string
		}{
			{
				name:     "nil type",
				typeInfo: nil,
				want:     "nil",
			},
			{
				name: "named type",
				typeInfo: &TypeInfo{
					Kind:       Named,
					Name:       "User",
					ImportPath: "github.com/example/pkg",
				},
				want: "github.com/example/pkg.User",
			},
			{
				name: "primitive type",
				typeInfo: &TypeInfo{
					Kind: Primitive,
					Name: "int",
				},
				want: "int",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.typeInfo.String(); got != tt.want {
					t.Errorf("TypeInfo.String() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	// Test equality
	t.Run("Equals", func(t *testing.T) {
		userType := &TypeInfo{
			Kind:       Struct,
			Name:       "User",
			ImportPath: "github.com/example/pkg",
			Fields: []*FieldInfo{
				{
					Name: "ID",
					Type: &TypeInfo{Kind: Primitive, Name: "int"},
				},
			},
		}

		tests := []struct {
			name     string
			typeInfo *TypeInfo
			other    *TypeInfo
			want     bool
		}{
			{
				name:     "both nil",
				typeInfo: nil,
				other:    nil,
				want:     true,
			},
			{
				name:     "one nil",
				typeInfo: userType,
				other:    nil,
				want:     false,
			},
			{
				name:     "same type",
				typeInfo: userType,
				other:    userType,
				want:     true,
			},
			{
				name:     "different name",
				typeInfo: userType,
				other: &TypeInfo{
					Kind:       Struct,
					Name:       "Product",
					ImportPath: "github.com/example/pkg",
					Fields:     userType.Fields,
				},
				want: false,
			},
			{
				name:     "different fields",
				typeInfo: userType,
				other: &TypeInfo{
					Kind:       Struct,
					Name:       "User",
					ImportPath: "github.com/example/pkg",
					Fields: []*FieldInfo{
						{
							Name: "Name",
							Type: &TypeInfo{Kind: Primitive, Name: "string"},
						},
					},
				},
				want: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.typeInfo.Equals(tt.other); got != tt.want {
					t.Errorf("TypeInfo.Equals() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	// Test package name caching
	t.Run("GetPackageName", func(t *testing.T) {
		ti := &TypeInfo{
			ImportPath: "github.com/example/pkg",
		}

		// First call should compute and cache
		pkg1 := ti.PackageName()
		if pkg1 != "pkg" {
			t.Errorf("Expected package name 'pkg', got '%s'", pkg1)
		}

		// Second call should use cache
		pkg2 := ti.PackageName()
		if pkg2 != "pkg" {
			t.Errorf("Expected cached package name 'pkg', got '%s'", pkg2)
		}
	})

	// Test FieldInfo equality
	t.Run("FieldInfo Equals", func(t *testing.T) {
		intType := &TypeInfo{Kind: Primitive, Name: "int"}
		stringType := &TypeInfo{Kind: Primitive, Name: "string"}

		field1 := &FieldInfo{
			Name:       "ID",
			Type:       intType,
			Tag:        `json:"id"`,
			IsEmbedded: false,
		}

		tests := []struct {
			name  string
			field *FieldInfo
			other *FieldInfo
			want  bool
		}{
			{
				name:  "both nil",
				field: nil,
				other: nil,
				want:  true,
			},
			{
				name:  "one nil",
				field: field1,
				other: nil,
				want:  false,
			},
			{
				name:  "same field",
				field: field1,
				other: field1,
				want:  true,
			},
			{
				name:  "different name",
				field: field1,
				other: &FieldInfo{
					Name:       "Name",
					Type:       stringType,
					Tag:        `json:"name"`,
					IsEmbedded: false,
				},
				want: false,
			},
			{
				name:  "different type",
				field: field1,
				other: &FieldInfo{
					Name:       "ID",
					Type:       stringType,
					Tag:        `json:"id"`,
					IsEmbedded: false,
				},
				want: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.field.Equals(tt.other); got != tt.want {
					t.Errorf("FieldInfo.Equals() = %v, want %v", got, tt.want)
				}
			})
		}
	})
}

// TestTypeInfoStringRepresentations tests the three string representation methods
func TestTypeInfoStringRepresentations(t *testing.T) {
	tests := []struct {
		name           string
		typeInfo       *TypeInfo
		wantFQN        string
		wantPackageDot string
		wantTypeOnly   string
	}{
		{
			name: "named type with import path",
			typeInfo: &TypeInfo{
				Kind:       Named,
				Name:       "User",
				ImportPath: "github.com/example/pkg",
			},
			wantFQN:        "github.com/example/pkg.User",
			wantPackageDot: "pkg.User",
			wantTypeOnly:   "User",
		},
		{
			name: "primitive type",
			typeInfo: &TypeInfo{
				Kind: Primitive,
				Name: "int",
			},
			wantFQN:        "",
			wantPackageDot: "int",
			wantTypeOnly:   "int",
		},
		{
			name: "struct type with import path",
			typeInfo: &TypeInfo{
				Kind:       Struct,
				Name:       "Product",
				ImportPath: "github.com/example/models",
			},
			wantFQN:        "github.com/example/models.Product",
			wantPackageDot: "models.Product",
			wantTypeOnly:   "Product",
		},
		{
			name: "type without import path",
			typeInfo: &TypeInfo{
				Kind: Primitive,
				Name: "string",
			},
			wantFQN:        "",
			wantPackageDot: "string",
			wantTypeOnly:   "string",
		},
		{
			name:           "nil type",
			typeInfo:       nil,
			wantFQN:        "",
			wantPackageDot: "nil",
			wantTypeOnly:   "nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test FQN
			if got := tt.typeInfo.FQN(); got != tt.wantFQN {
				t.Errorf("TypeInfo.FQN() = %v, want %v", got, tt.wantFQN)
			}

			// Test PackageDotType
			if got := tt.typeInfo.TypeString(); got != tt.wantPackageDot {
				t.Errorf("TypeInfo.PackageDotType() = %v, want %v", got, tt.wantPackageDot)
			}

			// Test TypeNameOnly
			if got := tt.typeInfo.Type(); got != tt.wantTypeOnly {
				t.Errorf("TypeInfo.TypeNameOnly() = %v, want %v", got, tt.wantTypeOnly)
			}

			// Test PackageName
			if tt.typeInfo != nil && tt.typeInfo.ImportPath != "" {
				expectedPkg := "pkg"
				if tt.name == "struct type with import path" {
					expectedPkg = "models"
				}
				if got := tt.typeInfo.PackageName(); got != expectedPkg {
					t.Errorf("TypeInfo.PackageName() = %v, want %v", got, expectedPkg)
				}
			}
		})
	}
}

// TestTypeInfoComplexTypes tests string representations for complex types
func TestTypeInfoComplexTypes(t *testing.T) {
	// Create base types
	userType := &TypeInfo{
		Kind:       Struct,
		Name:       "User",
		ImportPath: "github.com/example/models",
	}
	userTypePtr := &TypeInfo{
		Kind:       Pointer,
		Underlying: userType,
	}

	intType := &TypeInfo{
		Kind: Primitive,
		Name: "int",
	}

	stringType := &TypeInfo{
		Kind: Primitive,
		Name: "string",
	}

	tests := []struct {
		name       string
		typeInfo   *TypeInfo
		wantGoType string
		wantString string
	}{
		{
			name: "pointer to struct",
			typeInfo: &TypeInfo{
				Kind:       Pointer,
				Underlying: userType,
			},
			wantGoType: "*models.User",
			wantString: "*models.User",
		},
		{
			name: "slice of struct",
			typeInfo: &TypeInfo{
				Kind:       Slice,
				Underlying: userTypePtr,
			},
			wantGoType: "[]*models.User",
			wantString: "[]*models.User",
		},
		{
			name: "array of int",
			typeInfo: &TypeInfo{
				Kind:       Array,
				ArrayLen:   5,
				Underlying: intType,
			},
			wantGoType: "[5]int",
			wantString: "[5]int",
		},
		{
			name: "map of string to struct",
			typeInfo: &TypeInfo{
				Kind:       Map,
				KeyType:    stringType,
				Underlying: userTypePtr,
			},
			wantGoType: "map[string]*models.User",
			wantString: "map[string]*models.User",
		},
		{
			name: "slice of slices",
			typeInfo: &TypeInfo{
				Kind: Slice,
				Underlying: &TypeInfo{
					Kind:       Slice,
					Underlying: userTypePtr,
				},
			},
			wantGoType: "[][]*models.User",
			wantString: "[][]*models.User",
		},
		{
			name: "pointer to slice",
			typeInfo: &TypeInfo{
				Kind: Pointer,
				Underlying: &TypeInfo{
					Kind:       Slice,
					Underlying: userTypePtr,
				},
			},
			wantGoType: "*[]*models.User",
			wantString: "*[]*models.User",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test TypeString
			if got := tt.typeInfo.TypeString(); got != tt.wantGoType {
				t.Errorf("TypeInfo.TypeString() = %v, want %v", got, tt.wantGoType)
			}

			// Test String method
			if got := tt.typeInfo.String(); got != tt.wantString {
				t.Errorf("TypeInfo.String() = %v, want %v", got, tt.wantString)
			}
		})
	}
}

// TestTypeInfoPackageNameExtraction tests package name extraction from various import paths
func TestTypeInfoPackageNameExtraction(t *testing.T) {
	tests := []struct {
		name        string
		importPath  string
		wantPkgName string
	}{
		{
			name:        "standard github path",
			importPath:  "github.com/example/models",
			wantPkgName: "models",
		},
		{
			name:        "golang.org path",
			importPath:  "golang.org/x/text/encoding",
			wantPkgName: "encoding",
		},
		{
			name:        "single segment",
			importPath:  "models",
			wantPkgName: "models",
		},
		{
			name:        "empty import path",
			importPath:  "",
			wantPkgName: "",
		},
		{
			name:        "path with version",
			importPath:  "github.com/example/pkg/v2",
			wantPkgName: "v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &TypeInfo{
				Kind:       Named,
				Name:       "TestType",
				ImportPath: tt.importPath,
			}

			if got := ti.PackageName(); got != tt.wantPkgName {
				t.Errorf("PackageName() for import path %q = %v, want %v", tt.importPath, got, tt.wantPkgName)
			}

			// Test caching by calling multiple times
			pkg1 := ti.PackageName()
			pkg2 := ti.PackageName()
			if pkg1 != pkg2 {
				t.Errorf("PackageName() should be consistent: first call %v, second call %v", pkg1, pkg2)
			}
		})
	}
}

// TestTypeInfoIsNamedType tests the IsNamedType method
func TestTypeInfoIsNamedType(t *testing.T) {
	tests := []struct {
		name     string
		typeInfo *TypeInfo
		want     bool
	}{
		{
			name: "named struct type",
			typeInfo: &TypeInfo{
				Kind:       Named,
				Name:       "User",
				ImportPath: "github.com/example/pkg",
			},
			want: true,
		},
		{
			name: "primitive type",
			typeInfo: &TypeInfo{
				Kind: Primitive,
				Name: "int",
			},
			want: false,
		},
		{
			name: "type without import path",
			typeInfo: &TypeInfo{
				Kind: Struct,
				Name: "User",
			},
			want: false,
		},
		{
			name: "type without name",
			typeInfo: &TypeInfo{
				Kind:       Struct,
				ImportPath: "github.com/example/pkg",
			},
			want: false,
		},
		{
			name:     "nil type",
			typeInfo: nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typeInfo.IsNamedType(); got != tt.want {
				t.Errorf("TypeInfo.IsNamedType() = %v, want %v", got, tt.want)
			}
		})
	}
}
