package components

import (
	"fmt"
	"strings"

	"github.com/origadmin/abgen/internal/model"
)

// TypeConverter implements the TypeConverter interface.
type TypeConverter struct{}

// NewTypeConverter creates a new type converter.
func NewTypeConverter() model.TypeConverter {
	return &TypeConverter{}
}

// Convert recursively builds a string representation of a type, using the provided
// pkgQualifier function to determine the correct package alias.
func (c *TypeConverter) Convert(typeInfo *model.TypeInfo, pkgQualifier func(string) string) string {
	if typeInfo == nil {
		return "nil"
	}

	var sb strings.Builder
	c.convertRecursive(&sb, typeInfo, pkgQualifier)
	return sb.String()
}

func (c *TypeConverter) convertRecursive(sb *strings.Builder, info *model.TypeInfo, pkgQualifier func(string) string) {
	switch info.Kind {
	case model.Pointer:
		sb.WriteString("*")
		c.convertRecursive(sb, info.Underlying, pkgQualifier)
	case model.Slice:
		sb.WriteString("[]")
		c.convertRecursive(sb, info.Underlying, pkgQualifier)
	case model.Array:
		sb.WriteString(fmt.Sprintf("[%d]", info.ArrayLen))
		c.convertRecursive(sb, info.Underlying, pkgQualifier)
	case model.Map:
		sb.WriteString("map[")
		c.convertRecursive(sb, info.KeyType, pkgQualifier)
		sb.WriteString("]")
		c.convertRecursive(sb, info.Underlying, pkgQualifier)
	case model.Named:
		if info.ImportPath != "" {
			alias := pkgQualifier(info.ImportPath)
			if alias != "" {
				sb.WriteString(alias)
				sb.WriteString(".")
			}
		}
		sb.WriteString(info.Name)
	case model.Primitive:
		sb.WriteString(info.Name)
	case model.Interface:
		sb.WriteString("interface{}")
	default:
		sb.WriteString("interface{}") // Fallback for safety
	}
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

// GetElementType returns the element type of pointers, slices, and arrays.
// It now correctly handles nested pointers and containers.
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

// GetKeyType returns the key type of a map.
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

// IsUltimatelyPrimitive checks if the given TypeInfo or its underlying type is ultimately a primitive type.
func (c *TypeConverter) IsUltimatelyPrimitive(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Primitive
}
