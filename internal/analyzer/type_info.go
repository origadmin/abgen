// Package analyzer is responsible for parsing Go source code, resolving types,
// and building a detailed internal representation of the packages and their types.
package analyzer

import (
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
	Name       string // The canonical name of the type (e.g., "User", "string", "[]*User", "*[]User")
	ImportPath string // The import path where this canonical type is defined

	Kind TypeKind // The fundamental kind of this type (e.g., Primitive, Struct, Slice, Pointer)

	IsPointer bool // Does this TypeInfo represent a pointer to another type? (e.g., *User)

	ArrayLen int // If Kind is Array, the length of the array.

	// --- Recursive Type Relationships (Unified) ---
	// This field points to the type that this TypeInfo is built upon.
	// - For 'type XXX YYY' or 'type XXX = YYY', it points to YYY.
	// - For '*T', it points to T.
	// - For '[]T', '[N]T', 'chan T', it points to T.
	// - For 'map[K]V', it points to V (the value type). KeyType is separate.
	Underlying *TypeInfo

	KeyType *TypeInfo // If Kind is Map, points to the TypeInfo of its key.

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
