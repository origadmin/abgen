// Package analyzer is responsible for parsing Go source code, resolving types,
// and building a detailed internal representation of the packages and their types.
package analyzer

import (
	"fmt"
	"go/types"
	"strings"
)

// TypeKind defines the kind of a Go type.
type TypeKind int

// Constants for the different kinds of types.
// The naming follows Go's idiomatic style for enums (e.g., used as analyzer.Struct).
const (
	Unknown TypeKind = iota
	Primitive
	Struct
	Interface
	Map
	Chan
	Func
	Slice
	Array
	Pointer
)

// TypeInfo represents the detailed information of a resolved Go type.
// It serves as the primary data structure during the analysis phase.
type TypeInfo struct {
	// Name is the simple, non-qualified name of the type (e.g., "User", "string").
	// For anonymous types (like `[]int` or `*int`), this field will be empty.
	Name       string
	ImportPath string // The import path where this canonical type is defined

	Kind TypeKind // The fundamental kind of this type (e.g., Primitive, Struct, Slice, Pointer)

	ArrayLen int // If Kind is Array, the length of the array.

	// --- Recursive Type Relationships (Unified) ---
	// This field points to the type that this TypeInfo is built upon.
	// - For 'type XXX YYY' or 'type XXX = YYY', it points to YYY.
	// - For '*T', it points to T.
	// - For '[]T', '[N]T', 'chan T', it points to T.
	// - For 'map[K]V', it points to V (the value type). KeyType is separate.
	Underlying    *TypeInfo
	UnderlyingFQN string

	// If Kind is Map, points to the TypeInfo of its key.
	KeyType    *TypeInfo
	KeyTypeFQN string

	// --- Alias-specific Information ---
	// Is this TypeInfo representing a type alias declaration (type XXX = YYY)?
	// If false, it's a new type definition (type XXX YYY).
	IsAlias bool

	// This field is currently used in config, can remain as a string from config.
	LocalAlias string

	// --- Struct-specific Fields ---

	// Fields contains the list of fields for a struct type.
	Fields []*FieldInfo

	// Methods contains the list of methods associated with the type.
	Methods []*MethodInfo

	// Original is the raw type object from the go/types package.
	// This can be used for more advanced type-checking or inspection if needed.
	Original types.Object
}

// GetName returns the base name of the type.
func (ti *TypeInfo) GetName() string {
	if ti == nil {
		return ""
	}
	parts := strings.Split(ti.Name, ".")
	return parts[len(parts)-1]
}

func (ti *TypeInfo) String() string {
	return ti.Name
}

func (ti *TypeInfo) FQN() string {
	return ti.ImportPath + "." + ti.Name
}

// ToTypeString reconstructs the Go type string from the TypeInfo.
// It handles named types, pointers, slices, arrays, and maps.
func (ti *TypeInfo) ToTypeString() string {
	if ti == nil {
		return "nil"
	}

	var sb strings.Builder

	// Handle pointer
	if ti.Kind == Pointer {
		sb.WriteString("*")
	}

	// Handle slice/array
	if ti.Kind == Slice {
		sb.WriteString("[]")
	} else if ti.Kind == Array {
		sb.WriteString(fmt.Sprintf("[%d]", ti.ArrayLen))
	}

	// Handle map
	if ti.Kind == Map {
		sb.WriteString("map[")
		if ti.KeyType != nil {
			sb.WriteString(ti.KeyType.ToTypeString())
		} else {
			sb.WriteString("?")
		}
		sb.WriteString("]")
	}

	// Handle the base type (named or primitive)
	if ti.Name != "" {
		// If it's a named type and has an import path, use FQN for clarity in reconstruction
		// Primitive types (like "int") don't need import path prefix, even if they have one (e.g., "builtin.int")
		if ti.ImportPath != "" && ti.Kind != Primitive {
			sb.WriteString(ti.ImportPath)
			sb.WriteString(".")
		}
		sb.WriteString(ti.Name)
	} else if ti.Underlying != nil { // For anonymous composite types, recurse to the underlying type
		// This handles cases like `[]*User` where `[]` is the outer TypeInfo,
		// and `*User` is its Underlying. The Name for the outer TypeInfo (Slice) would be empty.
		sb.WriteString(ti.Underlying.ToTypeString())
	} else {
		// Fallback for unhandled or truly anonymous types without an underlying
		// This case should ideally not be hit if all types are properly handled.
		sb.WriteString(ti.Kind.String())
	}

	return sb.String()
}

// FieldInfo represents a single field within a struct.
type FieldInfo struct {
	// Name is the name of the field.
	Name string

	// Type is the resolved type information for the field.
	Type *TypeInfo

	// Tag is the struct tag string associated with the field.
	Tag string

	// IsEmbedded indicates whether the field is an embedded field.
	IsEmbedded bool
}

// MethodInfo represents a single method of a type.
type MethodInfo struct {
	// Name is the name of the method.
	Name string

	// Signature is the signature of the method.
	Signature *types.Signature
}

func (k TypeKind) String() string {
	switch k {
	case Primitive:
		return "Primitive"
	case Struct:
		return "Struct"
	case Interface:
		return "Interface"
	case Map:
		return "Map"
	case Chan:
		return "Chan"
	case Func:
		return "Func"
	case Slice:
		return "Slice"
	case Array:
		return "Array"
	case Pointer:
		return "Pointer"
	default:
		return "Unknown"
	}
}
