// Package generator provides utilities for converting between different type representations.
package generator

import (
	"github.com/origadmin/abgen/internal/analyzer"
	"github.com/origadmin/abgen/internal/model"
)

// TypeConverter handles conversion from analyzer.TypeInfo to model.Type.
type TypeConverter struct {
	cache map[*analyzer.TypeInfo]*model.Type
}

// NewTypeConverter creates a new TypeConverter.
func NewTypeConverter() *TypeConverter {
	return &TypeConverter{
		cache: make(map[*analyzer.TypeInfo]*model.Type),
	}
}

// ConvertToModel converts analyzer.TypeInfo to model.Type.
func (c *TypeConverter) ConvertToModel(info *analyzer.TypeInfo) *model.Type {
	if info == nil {
		return nil
	}
	
	// Check cache
	if result, exists := c.cache[info]; exists {
		return result
	}
	
	// Create new model type
	modelType := &model.Type{
		Name:       info.Name,
		ImportPath: info.ImportPath,
		IsPointer:  info.Kind == analyzer.Pointer,
		Fields:     make([]*model.Field, 0),
	}

	
	// IMPORTANT: Add to cache immediately to prevent infinite recursion for self-referencing types
	c.cache[info] = modelType 
	
	// Convert kind
	switch info.Kind {
	case analyzer.Struct:
		modelType.Kind = model.TypeKindStruct
	case analyzer.Slice:
		modelType.Kind = model.TypeKindSlice
	case analyzer.Pointer:
		modelType.Kind = model.TypeKindPointer
	case analyzer.Primitive:
		modelType.Kind = model.TypeKindPrimitive
	case analyzer.Interface:
		modelType.Kind = model.TypeKindInterface
	case analyzer.Map:
		modelType.Kind = model.TypeKindMap
	default:
		modelType.Kind = model.TypeKindUnknown
	}
	
	// Convert element type for slices, pointers, maps (use Underlying field)
	if info.Underlying != nil {
		modelType.ElementType = c.ConvertToModel(info.Underlying)
	}
	
	// Convert key type for maps
	if info.KeyType != nil {
		modelType.KeyType = c.ConvertToModel(info.KeyType)
	}
	
	// Convert fields for structs
	for _, field := range info.Fields {
		modelField := &model.Field{
			Name:       field.Name,
			Type:       c.ConvertToModel(field.Type),
			Tag:        field.Tag,
			IsEmbedded: field.IsEmbedded,
		}
		modelType.Fields = append(modelType.Fields, modelField)
	}
	
	// No need to insert into cache again here, it's already there
	return modelType
}
