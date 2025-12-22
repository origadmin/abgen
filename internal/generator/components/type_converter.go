package components

import (
	"github.com/origadmin/abgen/internal/model"
)

// TypeConverter implements the TypeConverter interface.
type TypeConverter struct{}

// NewTypeConverter creates a new type converter.
func NewTypeConverter() model.TypeConverter {
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

// GetElementType returns the element type of pointers, slices, and arrays.
// It correctly handles nested pointers and containers.
func (c *TypeConverter) GetElementType(info *model.TypeInfo) *model.TypeInfo {
	if info == nil {
		return nil
	}
	// This loop will "unwrap" any number of pointers, slices, or arrays
	// to find the ultimate base element.
	for {
		switch info.Kind {
		case model.Pointer, model.Slice, model.Array:
			info = info.Underlying
		default:
			// Once we hit a non-container type, we've found the element.
			return info
		}
	}
}

// GetSliceElementType returns the element type of a slice.
func (c *TypeConverter) GetSliceElementType(info *model.TypeInfo) *model.TypeInfo {
	info = c.resolveConcreteType(info)
	if info != nil && info.Kind == model.Slice {
		return info.Underlying
	}
	return nil
}

// GetKeyType returns the key type of a map.
func (c *TypeConverter) GetKeyType(info *model.TypeInfo) *model.TypeInfo {
	info = c.resolveConcreteType(info)
	if info != nil && info.Kind == model.Map {
		return info.KeyType
	}
	return nil
}

// IsUltimatelyPrimitive checks if the given TypeInfo or its underlying type is ultimately a primitive type.
func (c *TypeConverter) IsUltimatelyPrimitive(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Primitive
}

// IsPurelyPrimitiveOrCompositeOfPrimitives checks if the type is a primitive or a composite type
// (slice, array, map, pointer) made entirely of primitives.
// It returns false if the type involves any Named types or Structs.
func (c *TypeConverter) IsPurelyPrimitiveOrCompositeOfPrimitives(info *model.TypeInfo) bool {
	if info == nil {
		return false
	}

	switch info.Kind {
	case model.Primitive:
		return true
	case model.Named, model.Struct:
		// If it's a named type (even if underlying is int) or a struct, it's NOT purely primitive.
		return false
	case model.Pointer, model.Slice, model.Array:
		return c.IsPurelyPrimitiveOrCompositeOfPrimitives(info.Underlying)
	case model.Map:
		return c.IsPurelyPrimitiveOrCompositeOfPrimitives(info.KeyType) &&
			c.IsPurelyPrimitiveOrCompositeOfPrimitives(info.Underlying)
	default:
		return false
	}
}
