package components

import (
	"testing"

	"github.com/origadmin/abgen/internal/model"
)

// Helper function to create a primitive type for tests
func newPrimitive(name string) *model.TypeInfo {
	return &model.TypeInfo{Name: name, Kind: model.Primitive}
}

// Helper function to create a named type for tests
func newNamed(name, path string, underlying *model.TypeInfo) *model.TypeInfo {
	return &model.TypeInfo{
		Name:       name,
		ImportPath: path,
		Kind:       model.Named,
		Underlying: underlying,
	}
}

// Helper function to create a pointer type for tests
func newPointer(elem *model.TypeInfo) *model.TypeInfo {
	return &model.TypeInfo{Kind: model.Pointer, Underlying: elem}
}

// Helper function to create a slice type for tests
func newSlice(elem *model.TypeInfo) *model.TypeInfo {
	return &model.TypeInfo{Kind: model.Slice, Underlying: elem}
}

// Helper function to create a struct type for tests
func newStruct(name, path string, fields []*model.FieldInfo) *model.TypeInfo {
	return &model.TypeInfo{
		Name:       name,
		ImportPath: path,
		Kind:       model.Struct,
		Fields:     fields,
	}
}

func TestTypeConverter_GetConcreteType(t *testing.T) {
	tc := NewTypeConverter()

	intType := newPrimitive("int")
	namedInt := newNamed("MyInt", "a/b", intType)
	namedNamedInt := newNamed("MyNamedInt", "a/b", namedInt)

	structType := newStruct("User", "a/b", nil)
	namedStruct := newNamed("MyUser", "a/b", structType)

	testCases := []struct {
		name     string
		input    *model.TypeInfo
		expected *model.TypeInfo
	}{
		{"Primitive", intType, intType},
		{"Named Primitive", namedInt, intType},
		{"Doubly Named Primitive", namedNamedInt, intType},
		{"Struct", structType, structType},
		{"Named Struct", namedStruct, structType},
		{"Pointer to Named", newPointer(namedStruct), newPointer(namedStruct)}, // GetConcreteType does not unwrap pointers
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got := tc.GetConcreteType(tt.input)
			if got.Kind != tt.expected.Kind || got.Name != tt.expected.Name {
				t.Errorf("GetConcreteType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTypeConverter_GetElementType(t *testing.T) {
	tc := NewTypeConverter()

	intType := newPrimitive("int")
	structType := newStruct("User", "a/b", nil)

	testCases := []struct {
		name     string
		input    *model.TypeInfo
		expected *model.TypeInfo
	}{
		{"Primitive", intType, intType},
		{"Pointer to Primitive", newPointer(intType), intType},
		{"Slice of Primitive", newSlice(intType), intType},
		{"Slice of Pointers", newSlice(newPointer(intType)), intType},
		{"Pointer to Slice", newPointer(newSlice(intType)), intType},
		{"Pointer to Slice of Pointers", newPointer(newSlice(newPointer(structType))), structType},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got := tc.GetElementType(tt.input)
			if got.Kind != tt.expected.Kind || got.Name != tt.expected.Name {
				t.Errorf("GetElementType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTypeConverter_IsUltimatelyPrimitive(t *testing.T) {
	tc := NewTypeConverter()

	intType := newPrimitive("int")
	namedInt := newNamed("MyInt", "a/b", intType)
	structType := newStruct("User", "a/b", nil)
	namedStruct := newNamed("MyUser", "a/b", structType)

	testCases := []struct {
		name     string
		input    *model.TypeInfo
		expected bool
	}{
		{"Primitive", intType, true},
		{"Named Primitive", namedInt, true},
		{"Struct", structType, false},
		{"Named Struct", namedStruct, false},
		{"Pointer to Primitive", newPointer(intType), false}, // IsUltimatelyPrimitive does not unwrap pointers/slices
		{"Slice of Primitive", newSlice(intType), false},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := tc.IsUltimatelyPrimitive(tt.input); got != tt.expected {
				t.Errorf("IsUltimatelyPrimitive() = %v, want %v", got, tt.expected)
			}
		})
	}
}
