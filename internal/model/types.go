package model

import (
	"fmt"
	"go/types"
	"log/slog"
	"strings"
)

// TypeKind defines the kind of a Go type.
type TypeKind int

// Constants for the different kinds of types.
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
	Defined // For defined types (type T U)
)

// TypeInfo represents the detailed information of a resolved Go type.
// It serves as the single, authoritative data model for types throughout the application.
type TypeInfo struct {
	Name         string
	ImportPath   string 
	Kind         TypeKind
	ArrayLen     int
	Underlying   *TypeInfo 
	KeyType      *TypeInfo
	IsAlias      bool
	Fields       []*FieldInfo
	Methods      []*MethodInfo
	Original     types.Object
}

// GetName returns the base name of the type.
func (ti *TypeInfo) GetName() string {
	if ti == nil {
		return ""
	}
	return ti.Name
}

func (ti *TypeInfo) String() string {
	return ti.Name
}

// IsNamedType returns true if the type has a name and an import path, and is not a primitive.
func (ti *TypeInfo) IsNamedType() bool {
	return ti != nil && ti.Name != "" && ti.ImportPath != "" && ti.Kind != Primitive
}

// FQN returns the Fully Qualified Name for named types.
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
func (ti *TypeInfo) GoTypeString() string {
	if ti == nil {
		slog.Debug("GoTypeString", "ti", "nil", "result", "nil")
		return "nil"
	}



	return ti.buildTypeStringFromUnderlying()
}

func (ti *TypeInfo) buildTypeStringFromUnderlying() string {
	var sb strings.Builder

	switch ti.Kind {
	case Pointer:
		sb.WriteString("*")
		if ti.Underlying != nil { 
			sb.WriteString(ti.Underlying.GoTypeString())
		} else {
			sb.WriteString("interface{}")
		}
	case Slice:
		sb.WriteString("[]")
		if ti.Underlying != nil { 
			sb.WriteString(ti.Underlying.GoTypeString())
		} else {
			sb.WriteString("interface{}")
		}
	case Array:
		sb.WriteString(fmt.Sprintf("[%d]", ti.ArrayLen))
		if ti.Underlying != nil { 
			sb.WriteString(ti.Underlying.GoTypeString())
		} else {
			sb.WriteString("interface{}")
		}
	case Map:
		sb.WriteString("map[")
		if ti.KeyType != nil {
			sb.WriteString(ti.KeyType.GoTypeString())
		} else {
			sb.WriteString("interface{}")
		}
		sb.WriteString("]")
		if ti.Underlying != nil { 
			sb.WriteString(ti.Underlying.GoTypeString())
		} else {
			sb.WriteString("interface{}")
		}
	case Defined:
		// For defined types, we should show the type name, not the underlying type
		// This preserves the semantic meaning of the defined type
		if ti.Name != "" {
			// For defined types from other packages, include the package name
			if ti.ImportPath != "" {
				// Extract package name from import path
				lastSlash := strings.LastIndex(ti.ImportPath, "/")
				packageName := ti.ImportPath[lastSlash+1:]
				sb.WriteString(packageName)
				sb.WriteString(".")
			}
			sb.WriteString(ti.Name)
		} else {
			sb.WriteString("interface{}")
		}
	case Primitive, Struct, Interface, Chan, Func:
		if ti.Name != "" {
			// For named types from other packages, include the package name
			if ti.ImportPath != "" {
				// Extract package name from import path
				lastSlash := strings.LastIndex(ti.ImportPath, "/")
				packageName := ti.ImportPath[lastSlash+1:]
				sb.WriteString(packageName)
				sb.WriteString(".")
			}
			sb.WriteString(ti.Name)
		} else {
			sb.WriteString("interface{}")
		}
	default:
		sb.WriteString("unknown")
	}

	return sb.String()
}

// FieldInfo represents a single field within a struct.
type FieldInfo struct {
	Name       string
	Type       *TypeInfo
	Tag        string
	IsEmbedded bool
	// IsExported bool // Removed IsExported field
}

// MethodInfo represents a single method of a type.
type MethodInfo struct {
	Name      string
	Signature *SignatureInfo
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
	case Defined:
		return "Defined"
	default:
		return "Unknown"
	}
}
