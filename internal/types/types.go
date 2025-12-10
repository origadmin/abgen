// Package types implements the functions, types, and interfaces for the module.
package types

import (
	"strings"
)

const (
	Application = "abgen"
	Description = "Alias Binding Generator is a tool for generating code for conversion between two types"
	WebSite     = "https://github.com/origadmin/abgen"
	UI          = `
   _____ ___. 
  /  _  \_ |__    ____   ____   ____
 /  /_\  \| __ \  / ___\_/ __ \ /    \
/    |    \ \_\ \/ /_/  >  ___/|   |  \
\____|__  /___  /\___  / \___  >___|  /
        \/    \//_____/      \/     \/
`
)

// TypeKind represents the fundamental kind of a Go type.
type TypeKind int

const (
	UnknownKind TypeKind = iota
	PrimitiveKind
	StructKind
	InterfaceKind
	MapKind
	ChanKind
	FuncKind
	SliceKind
	ArrayKind
	PointerKind
)

// StructField represents a field in a struct.
type StructField struct {
	Name     string
	Type     string    // The string representation of the field's type (e.g., "int", "*User", "[]string")
	TypeInfo *TypeInfo // Resolved TypeInfo for this field's type. This is crucial.
	Tags     string
}

// TypeInfo holds resolved information about a Go type.
type TypeInfo struct {
	Name       string // The canonical name of the type (e.g., "User", "string", "[]*User", "*[]User")
	ImportPath string // The import path where this canonical type is defined

	Kind TypeKind // The fundamental kind of this type (e.g., PrimitiveKind, StructKind, SliceKind, PointerKind)

	IsPointer bool // Does this TypeInfo represent a pointer to another type? (e.g., *User)

	ArrayLen int // If Kind is ArrayKind, the length of the array.

	// --- Recursive Type Relationships (Unified) ---
	// This field points to the type that this TypeInfo is built upon.
	// - For 'type XXX YYY' or 'type XXX = YYY', it points to YYY.
	// - For '*T', it points to T.
	// - For '[]T', '[N]T', 'chan T', it points to T.
	// - For 'map[K]V', it points to V (the value type). KeyType is separate.
	Underlying *TypeInfo 

	KeyType    *TypeInfo // If Kind is MapKind, points to the TypeInfo of its key.

	// --- Alias-specific Information ---
	IsAlias bool // Is this TypeInfo representing a type alias declaration (type XXX = YYY)?
	        // If false, it's a new type definition (type XXX YYY).

	LocalAlias string // This field is currently used in config, can remain as a string from config.

	// --- Struct-specific Fields ---
	Fields []StructField // If Kind is StructKind, lists the fields of the struct.
}

// GetName returns the base name of the type.
func (ti *TypeInfo) GetName() string {
	if ti == nil {
		return ""
	}
	parts := strings.Split(ti.Name, ".")
	return parts[len(parts)-1]
}

// Import represents a single Go import statement.
type Import struct {
	Alias string
	Path  string
}

// ImportManager defines the interface for managing imports and aliases during code generation.
type ImportManager interface {
	GetType(pkgPath, typeName string) string
	GetImports() []Import
	RegisterAlias(alias string)
	IsAliasRegistered(alias string) bool
}

// IsPrimitiveType checks if a type name is a Go primitive type.
func IsPrimitiveType(t string) bool {
	primitiveTypes := map[string]bool{
		"bool":   true,
		"string": true,
		"int":    true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
		"byte": true, "rune": true,
	}
	return primitiveTypes[t]
}

// IsNumberType checks if a type name is a numeric type.
func IsNumberType(t string) bool {
	numberTypes := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
	}
	return numberTypes[t]
}
