// Package generator provides utilities for type conversions.
package generator

import (
	"github.com/origadmin/abgen/internal/model"
)

// TypeConverter handles type conversions and utility functions.
// This is now a utility class that works directly with model.TypeInfo.
type TypeConverter struct {
	// Cache can be added if needed for performance optimization
}

// NewTypeConverter creates a new TypeConverter.
func NewTypeConverter() *TypeConverter {
	return &TypeConverter{}
}

// resolveConcreteType traverses the 'Underlying' chain of a TypeInfo
// until it finds a non-Named type, which represents the concrete physical type.
func (c *TypeConverter) resolveConcreteType(info *model.TypeInfo) *model.TypeInfo {
	for info != nil && info.Kind == model.Named {
		info = info.Underlying
	}
	return info
}

// IsPointer checks if the given TypeInfo represents a pointer type.
func (c *TypeConverter) IsPointer(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Pointer
}

// GetElementType returns the element type for pointers, slices, and arrays.
func (c *TypeConverter) GetElementType(info *model.TypeInfo) *model.TypeInfo {
	info = c.resolveConcreteType(info)
	if info == nil {
		return nil
	}

	switch info.Kind {
	case model.Pointer, model.Slice, model.Array:
		return info.Underlying
	default:
		return nil
	}
}

// GetKeyType returns the key type for maps.
func (c *TypeConverter) GetKeyType(info *model.TypeInfo) *model.TypeInfo {
	info = c.resolveConcreteType(info)
	if info != nil && info.Kind == model.Map {
		return info.KeyType
	}
	return nil
}

// IsStruct checks if the given TypeInfo represents a struct type.
func (c *TypeConverter) IsStruct(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Struct
}

// IsSlice checks if the given TypeInfo represents a slice type.
func (c *TypeConverter) IsSlice(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Slice
}

// IsArray checks if the given TypeInfo represents an array type.
func (c *TypeConverter) IsArray(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Array
}

// IsMap checks if the given TypeInfo represents a map type.
func (c *TypeConverter) IsMap(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Map
}

// IsPrimitive checks if the given TypeInfo represents a primitive type.
func (c *TypeConverter) IsPrimitive(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Primitive
}
