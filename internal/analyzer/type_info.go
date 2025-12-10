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
	// For composite types, this might be a synthetic name like "[]int" or "*string".
	Name       string
	ImportPath string // The import path where this canonical type is defined

	Kind TypeKind // The fundamental kind of this type

	ArrayLen int // If Kind is Array, the length of the array.

	// Underlying points to the type that this TypeInfo is built upon.
	// - For 'type XXX YYY' or 'type XXX = YYY', it points to YYY.
	// - For '*T', it points to T.
	// - For '[]T', '[N]T', 'chan T', it points to T.
	// - For 'map[K]V', it points to V (the value type). KeyType is separate.
	Underlying *TypeInfo

	// If Kind is Map, points to the TypeInfo of its key.
	KeyType *TypeInfo

	// IsAlias indicates if this TypeInfo represents a type alias declaration (type XXX = YYY).
	IsAlias bool

	// Fields contains the list of fields for a struct type.
	Fields []*FieldInfo

	// Methods contains the list of methods associated with the type.
	Methods []*MethodInfo // Changed to use SignatureInfo

	// Original is the raw type object from the go/types package.
	Original types.Object
}

// GetName returns the base name of the type.
func (ti *TypeInfo) GetName() string {
	if ti == nil {
		return ""
	}
	return ti.Name // Name is now expected to be the simple name
}

func (ti *TypeInfo) String() string {
	return ti.Name
}

// IsNamedType returns true if the type has a name and an import path, and is not a primitive.
func (ti *TypeInfo) IsNamedType() bool {
	return ti != nil && ti.Name != "" && ti.ImportPath != "" && ti.Kind != Primitive
}

// FQN returns the Fully Qualified Name for named types.
// For anonymous types, it returns an empty string.
func (ti *TypeInfo) FQN() string {
	if ti == nil {
		return ""
	}
	if ti.IsNamedType() {
		return ti.ImportPath + "." + ti.Name
	}
	return ""
}

// GoTypeString reconstructs the Go type string from the TypeInfo, suitable for code generation.
// It does not include import paths; the generator is responsible for managing imports and aliases.
func (ti *TypeInfo) GoTypeString() string {
	if ti == nil {
		return "nil"
	}

	var sb strings.Builder

	switch ti.Kind {
	case Pointer:
		sb.WriteString("*")
		if ti.Underlying != nil {
			sb.WriteString(ti.Underlying.GoTypeString())
		} else {
			sb.WriteString("interface{}") // Fallback for unknown underlying type
		}
	case Slice:
		sb.WriteString("[]")
		if ti.Underlying != nil {
			sb.WriteString(ti.Underlying.GoTypeString())
		} else {
			sb.WriteString("interface{}") // Fallback for unknown element type
		}
	case Array:
		sb.WriteString(fmt.Sprintf("[%d]", ti.ArrayLen))
		if ti.Underlying != nil {
			sb.WriteString(ti.Underlying.GoTypeString())
		} else {
			sb.WriteString("interface{}") // Fallback for unknown element type
		}
	case Map:
		sb.WriteString("map[")
		if ti.KeyType != nil {
			sb.WriteString(ti.KeyType.GoTypeString())
		} else {
			sb.WriteString("interface{}") // Fallback for unknown key type
		}
		sb.WriteString("]")
		if ti.Underlying != nil { // Underlying is the value type for map
			sb.WriteString(ti.Underlying.GoTypeString())
		} else {
			sb.WriteString("interface{}") // Fallback for unknown value type
		}
	case Primitive, Struct, Interface, Chan, Func:
		// For named types (including primitives), use the simple name.
		// Import path handling (e.g., adding package alias) is the responsibility of the code generator.
		sb.WriteString(ti.Name)
	default:
		sb.WriteString("unknown") // Fallback for unhandled kinds
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
	Signature *SignatureInfo // Changed to use SignatureInfo
}

// SignatureInfo represents the detailed information of a function or method signature.
type SignatureInfo struct {
	Params     []*TypeInfo
	Results    []*TypeInfo
	IsVariadic bool
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
