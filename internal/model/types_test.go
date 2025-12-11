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
				name: "different name",
				typeInfo: userType,
				other: &TypeInfo{
					Kind:       Struct,
					Name:       "Product",
					ImportPath: "github.com/example/pkg",
					Fields: userType.Fields,
				},
				want: false,
			},
			{
				name: "different fields",
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
		pkg1 := ti.getPackageName()
		if pkg1 != "pkg" {
			t.Errorf("Expected package name 'pkg', got '%s'", pkg1)
		}

		// Second call should use cache
		pkg2 := ti.getPackageName()
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
			name     string
			field    *FieldInfo
			other    *FieldInfo
			want     bool
		}{
			{
				name:     "both nil",
				field:    nil,
				other:    nil,
				want:     true,
			},
			{
				name:     "one nil",
				field:    field1,
				other:    nil,
				want:     false,
			},
			{
				name:     "same field",
				field:    field1,
				other:    field1,
				want:     true,
			},
			{
				name: "different name",
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
				name: "different type",
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