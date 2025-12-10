package generator

import (
	"testing"

	"github.com/origadmin/abgen/internal/analyzer"
	"github.com/origadmin/abgen/internal/model"
)

func TestTypeConverter_ConvertToModel(t *testing.T) {
	converter := NewTypeConverter()
	
	// Test converting a basic struct type
	sourceInfo := &analyzer.TypeInfo{
		Name:        "User",
		ImportPath: "example.com/user",
		Kind:        analyzer.Struct,
		Fields: []*analyzer.FieldInfo{
			{
				Name: "ID",
				Type: &analyzer.TypeInfo{
					Name: "int",
					Kind: analyzer.Primitive,
				},
			},
			{
				Name: "Name",
				Type: &analyzer.TypeInfo{
					Name: "string",
					Kind: analyzer.Primitive,
				},
			},
		},
	}
	
	modelType := converter.ConvertToModel(sourceInfo)
	
	if modelType == nil {
		t.Fatal("ConvertToModel returned nil")
	}
	
	if modelType.Name != "User" {
		t.Errorf("Expected name 'User', got '%s'", modelType.Name)
	}
	
	if modelType.ImportPath != "example.com/user" {
		t.Errorf("Expected import path 'example.com/user', got '%s'", modelType.ImportPath)
	}
	
	if modelType.Kind != model.TypeKindStruct {
		t.Errorf("Expected kind %d, got %d", model.TypeKindStruct, modelType.Kind)
	}
	
	if len(modelType.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(modelType.Fields))
	}
	
	// Test field conversion
	idField := modelType.Fields[0]
	if idField.Name != "ID" {
		t.Errorf("Expected field name 'ID', got '%s'", idField.Name)
	}
	
	if idField.Type.Name != "int" {
		t.Errorf("Expected field type 'int', got '%s'", idField.Type.Name)
	}
}

func TestTypeConverter_Caching(t *testing.T) {
	converter := NewTypeConverter()
	
	sourceInfo := &analyzer.TypeInfo{
		Name:        "User",
		ImportPath: "example.com/user",
		Kind:        analyzer.Struct,
	}
	
	// Convert twice
	modelType1 := converter.ConvertToModel(sourceInfo)
	modelType2 := converter.ConvertToModel(sourceInfo)
	
	// Should return the same instance (cached)
	if modelType1 != modelType2 {
		t.Error("ConvertToModel should return cached result for same input")
	}
}

func TestTypeConverter_NilInput(t *testing.T) {
	converter := NewTypeConverter()
	
	result := converter.ConvertToModel(nil)
	if result != nil {
		t.Error("ConvertToModel should return nil for nil input")
	}
}