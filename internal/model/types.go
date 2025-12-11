package model

import (
	"fmt"
	"go/types"
	"log/slog"
	"strings"
	"sync"
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
	Named     // For any type introduced with the 'type' keyword
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
	
	// Cache for package name extraction
	packageNameMu sync.RWMutex
	packageName   string
}

// String returns a string representation of the type.
func (ti *TypeInfo) String() string {
	if ti == nil {
		return "nil"
	}
	if ti.IsNamedType() {
		return ti.FQN()
	}
	return ti.GoTypeString()
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

// IsValid checks if the TypeInfo contains valid data.
func (ti *TypeInfo) IsValid() bool {
	if ti == nil {
		return false
	}
	
	// Basic validation
	if ti.Kind == Unknown {
		return false
	}
	
	// Type-specific validation
	switch ti.Kind {
	case Array:
		return ti.ArrayLen > 0 && ti.Underlying != nil
	case Map:
		return ti.KeyType != nil && ti.Underlying != nil
	case Pointer, Slice:
		return ti.Underlying != nil
	case Struct, Interface, Chan, Func:
		return true // These can exist without underlying types
	case Named:
		return ti.IsNamedType()
	default:
		return ti.Kind == Primitive && ti.Name != ""
	}
}

// Equals checks if two TypeInfo objects represent the same type.
func (ti *TypeInfo) Equals(other *TypeInfo) bool {
	if ti == nil && other == nil {
		return true
	}
	if ti == nil || other == nil {
		return false
	}
	
	// Quick check for basic properties
	if ti.Name != other.Name || 
	   ti.ImportPath != other.ImportPath || 
	   ti.Kind != other.Kind ||
	   ti.IsAlias != other.IsAlias {
		return false
	}
	
	// For named types (Kind == Named), FQN comparison is sufficient
	if ti.Kind == Named && other.Kind == Named {
		return ti.FQN() == other.FQN()
	}
	
	// For complex types, compare structure
	switch ti.Kind {
	case Array:
		return ti.ArrayLen == other.ArrayLen && 
		       ti.Underlying.Equals(other.Underlying)
	case Map:
		return ti.KeyType.Equals(other.KeyType) && 
		       ti.Underlying.Equals(other.Underlying)
	case Pointer, Slice:
		return ti.Underlying.Equals(other.Underlying)
	case Struct:
		if len(ti.Fields) != len(other.Fields) {
			return false
		}
		for i, field := range ti.Fields {
			if !field.Equals(other.Fields[i]) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

// GoTypeString reconstructs the Go type string from the TypeInfo, suitable for code generation.
func (ti *TypeInfo) GoTypeString() string {
	if ti == nil {
		slog.Debug("GoTypeString", "ti", "nil", "result", "nil")
		return "nil"
	}



	return ti.buildTypeStringFromUnderlying()
}

// buildQualifiedTypeName builds the qualified type name with package prefix if needed.
func (ti *TypeInfo) buildQualifiedTypeName(sb *strings.Builder) {
	if ti.Name == "" {
		sb.WriteString("interface{}")
		return
	}
	
	if ti.ImportPath != "" {
		packageName := ti.getPackageName()
		sb.WriteString(packageName)
		sb.WriteString(".")
	}
	sb.WriteString(ti.Name)
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
	case Chan:
		if ti.Name != "" {
			ti.buildQualifiedTypeName(&sb)
		} else {
			sb.WriteString("chan interface{}")
		}
	case Func:
		if ti.Name != "" {
			ti.buildQualifiedTypeName(&sb)
		} else {
			sb.WriteString("func()")
		}
	case Named:
		// For named types, we should show the type name, not the underlying type
		// This preserves the semantic meaning of the defined type
		ti.buildQualifiedTypeName(&sb)
	case Primitive, Struct, Interface:
		ti.buildQualifiedTypeName(&sb)
	default:
		sb.WriteString("unknown")
	}

	return sb.String()
}

// getPackageName extracts and caches the package name from import path.
func (ti *TypeInfo) getPackageName() string {
	if ti.ImportPath == "" {
		return ""
	}
	
	// Use read lock first
	ti.packageNameMu.RLock()
	if ti.packageName != "" {
		defer ti.packageNameMu.RUnlock()
		return ti.packageName
	}
	ti.packageNameMu.RUnlock()
	
	// Acquire write lock for initialization
	ti.packageNameMu.Lock()
	defer ti.packageNameMu.Unlock()
	
	// Double-check after acquiring write lock
	if ti.packageName != "" {
		return ti.packageName
	}
	
	// Extract package name
	lastSlash := strings.LastIndex(ti.ImportPath, "/")
	if lastSlash == -1 {
		ti.packageName = ti.ImportPath
	} else {
		ti.packageName = ti.ImportPath[lastSlash+1:]
	}
	
	return ti.packageName
}

// FieldInfo represents a single field within a struct.
type FieldInfo struct {
	Name       string
	Type       *TypeInfo
	Tag        string
	IsEmbedded bool
	// IsExported bool // Removed IsExported field
}

// Equals checks if two FieldInfo objects are equal.
func (fi *FieldInfo) Equals(other *FieldInfo) bool {
	if fi == nil && other == nil {
		return true
	}
	if fi == nil || other == nil {
		return false
	}
	
	return fi.Name == other.Name &&
	       fi.Tag == other.Tag &&
	       fi.IsEmbedded == other.IsEmbedded &&
	       ((fi.Type == nil && other.Type == nil) || 
	        (fi.Type != nil && other.Type != nil && fi.Type.Equals(other.Type)))
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
	case Named:
		return "Named"
	default:
		return "Unknown"
	}
}
