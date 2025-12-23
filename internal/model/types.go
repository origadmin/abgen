package model

import (
	"fmt"
	"go/types"
	"log/slog"
	"strings"

	"github.com/origadmin/abgen/internal/config"
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
	Named // For any type introduced with the 'type' keyword
)

// AnalysisResult holds all the information gathered during the analysis phase
// and the final execution plan for the generator.
type AnalysisResult struct {
	// Initial analysis data
	TypeInfos         map[string]*TypeInfo
	ExistingFunctions map[string]bool
	ExistingAliases   map[string]string

	// The final plan for execution
	ExecutionPlan *ExecutionPlan
}

// ExecutionPlan contains the finalized configuration and rules for generation.
type ExecutionPlan struct {
	FinalConfig *config.Config
	ActiveRules []*config.ConversionRule
}

// Helper represents a built-in conversion function.
type Helper struct {
	Name         string
	SourceType   string
	TargetType   string
	Body         string
	Dependencies []string
}

// TypeInfo represents the detailed information of a resolved Go type.
// It serves as the single, authoritative data model for types throughout the application.
type TypeInfo struct {
	Name       string
	ImportPath string
	Kind       TypeKind
	ArrayLen   int
	Underlying *TypeInfo
	KeyType    *TypeInfo
	IsAlias    bool
	Fields     []*FieldInfo
	Methods    []*MethodInfo
	Original   types.Object
}

// ConversionTask represents a task for the code generator to create a conversion function.
type ConversionTask struct {
	Source *TypeInfo
	Target *TypeInfo
	Rule   *config.ConversionRule
}

// PackageName returns the package name of the type.
func (ti *TypeInfo) PackageName() string {
	parts := strings.Split(ti.ImportPath, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func (ti *TypeInfo) Type() string {
	if ti == nil {
		return "nil"
	}
	return ti.Name
}

func (ti *TypeInfo) OriginalType() types.Type {
	return ti.Original.Type()
}

// String returns a string representation of the type.
func (ti *TypeInfo) String() string {
	if ti == nil {
		return "nil"
	}
	if ti.IsNamedType() {
		return ti.FQN()
	}
	return ti.TypeString()
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

	if ti.Kind == Unknown {
		return false
	}

	switch ti.Kind {
	case Array:
		return ti.ArrayLen > 0 && ti.Underlying != nil
	case Map:
		return ti.KeyType != nil && ti.Underlying != nil
	case Pointer, Slice:
		return ti.Underlying != nil
	case Struct, Interface, Chan, Func:
		return true
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

	if ti.Name != other.Name ||
		ti.ImportPath != other.ImportPath ||
		ti.Kind != other.Kind ||
		ti.IsAlias != other.IsAlias {
		return false
	}

	if ti.Kind == Named && other.Kind == Named {
		return ti.FQN() == other.FQN()
	}

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

// TypeString reconstructs the Go type string from the TypeInfo, suitable for code generation.
func (ti *TypeInfo) TypeString() string {
	if ti == nil {
		slog.Debug("TypeString", "ti", "nil", "result", "nil")
		return "nil"
	}
	return ti.buildTypeStringFromUnderlying()
}

// BuildQualifiedTypeName builds the qualified type name with package prefix if needed.
func (ti *TypeInfo) BuildQualifiedTypeName(sb *strings.Builder) {
	if ti.Name == "" {
		sb.WriteString("interface{}")
		return
	}

	if ti.ImportPath != "" {
		sb.WriteString(ti.PackageName())
		sb.WriteString(".")
	}
	sb.WriteString(ti.Name)
}

// IsUltimatelyStruct checks if the type is a struct or a named type whose underlying type is a struct.
func (ti *TypeInfo) IsUltimatelyStruct() bool {
	if ti == nil {
		return false
	}
	if ti.Kind == Struct {
		return true
	}
	if ti.Kind == Named && ti.Underlying != nil {
		return ti.Underlying.IsUltimatelyStruct()
	}
	return false
}

// FieldInfo represents a single field within a struct.
type FieldInfo struct {
	Name       string
	Type       *TypeInfo
	Tag        string
	IsEmbedded bool
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

// UniqueKey returns a string that uniquely identifies the type.
func (ti *TypeInfo) UniqueKey() string {
	if ti == nil {
		return ""
	}

	if ti.ImportPath != "" && ti.Name != "" {
		return ti.FQN()
	}

	return ti.buildTypeString(true)
}

func (ti *TypeInfo) buildTypeStringFromUnderlying() string {
	return ti.buildTypeString(false)
}

func (ti *TypeInfo) buildTypeString(isUniqueKeyMode bool) string {
	if ti == nil {
		return "nil"
	}

	if isUniqueKeyMode && ti.ImportPath != "" && ti.Name != "" {
		return ti.FQN()
	}

	var sb strings.Builder

	switch ti.Kind {
	case Pointer:
		sb.WriteString("*")
		sb.WriteString(ti.getTypeRepresentation(ti.Underlying, isUniqueKeyMode))
	case Slice:
		sb.WriteString("[]")
		sb.WriteString(ti.getTypeRepresentation(ti.Underlying, isUniqueKeyMode))
	case Array:
		sb.WriteString(fmt.Sprintf("[%d]", ti.ArrayLen))
		sb.WriteString(ti.getTypeRepresentation(ti.Underlying, isUniqueKeyMode))
	case Map:
		sb.WriteString("map[")
		sb.WriteString(ti.getTypeRepresentation(ti.KeyType, isUniqueKeyMode))
		sb.WriteString("]")
		sb.WriteString(ti.getTypeRepresentation(ti.Underlying, isUniqueKeyMode))
	case Chan:
		if ti.Name != "" {
			ti.buildTypeName(&sb, isUniqueKeyMode)
		} else {
			sb.WriteString("chan interface{}")
		}
	case Func:
		if ti.Name != "" {
			ti.buildTypeName(&sb, isUniqueKeyMode)
		} else {
			sb.WriteString("func()")
		}
	case Named:
		ti.buildTypeName(&sb, isUniqueKeyMode)
	case Primitive:
		return ti.Name
	case Struct, Interface:
		ti.buildTypeName(&sb, isUniqueKeyMode)
	default:
		if isUniqueKeyMode {
			if ti.Name != "" {
				return ti.Name
			}
			slog.Warn("UniqueKey for unhandled TypeInfo kind", "kind", ti.Kind.String(), "info", fmt.Sprintf("%+v", ti))
			return fmt.Sprintf("unhandled_%s_%p", ti.Kind.String(), ti)
		}
		sb.WriteString("unknown")
	}

	return sb.String()
}

func (ti *TypeInfo) getTypeRepresentation(target *TypeInfo, isUniqueKeyMode bool) string {
	if target == nil {
		return "interface{}"
	}
	if isUniqueKeyMode {
		return target.UniqueKey()
	}
	return target.TypeString()
}

func (ti *TypeInfo) buildTypeName(sb *strings.Builder, isUniqueKeyMode bool) {
	if ti.Name == "" {
		sb.WriteString("interface{}")
		return
	}

	if isUniqueKeyMode {
		if ti.ImportPath != "" {
			sb.WriteString(ti.ImportPath)
			sb.WriteString(".")
		}
	} else {
		if ti.ImportPath != "" {
			sb.WriteString(ti.PackageName())
			sb.WriteString(".")
		}
	}
	sb.WriteString(ti.Name)
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

// GenerateUniqueKeyFromGoType creates a unique string key for a given Go types.Type.
func GenerateUniqueKeyFromGoType(goType types.Type) string {
	var sb strings.Builder
	generateUniqueKeyRecursive(goType, &sb)
	return sb.String()
}

func generateUniqueKeyRecursive(goType types.Type, sb *strings.Builder) {
	switch t := goType.(type) {
	case *types.Basic:
		sb.WriteString(t.String())
	case *types.Named:
		if t.Obj().Pkg() != nil {
			sb.WriteString(t.Obj().Pkg().Path())
			sb.WriteString(".")
		}
		sb.WriteString(t.Obj().Name())
	case *types.Pointer:
		sb.WriteString("*")
		generateUniqueKeyRecursive(t.Elem(), sb)
	case *types.Slice:
		sb.WriteString("[]")
		generateUniqueKeyRecursive(t.Elem(), sb)
	case *types.Array:
		sb.WriteString(fmt.Sprintf("[%d]", t.Len()))
		generateUniqueKeyRecursive(t.Elem(), sb)
	case *types.Map:
		sb.WriteString("map[")
		generateUniqueKeyRecursive(t.Key(), sb)
		sb.WriteString("]")
		generateUniqueKeyRecursive(t.Elem(), sb)
	case *types.Struct:
		sb.WriteString("struct{}")
	case *types.Interface:
		sb.WriteString("interface{}")
	case *types.Signature:
		sb.WriteString("func()")
	default:
		sb.WriteString(t.String())
	}
}
