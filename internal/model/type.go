// Package model defines abstract, simplified models used by the code generator.
// These models hide the complexity of the underlying go/types representation.
package model

// Type represents an abstract type model for code generation.
type Type struct {
	// Name is the simple name of the type.
	Name string
	// ImportPath is the import path where the type is defined.
	ImportPath string
	// Kind is the kind of the type.
	Kind TypeKind
	// IsPointer indicates if the type is a pointer.
	IsPointer bool
	// ElementType is the element type for pointers, slices, arrays.
	ElementType *Type
	// KeyType is the key type for maps.
	KeyType *Type
	// Fields contains the fields for struct types.
	Fields []*Field
}

// GetFQN returns the fully qualified name of the type.
func (t *Type) GetFQN() string {
	if t.ImportPath != "" {
		return t.ImportPath + "." + t.Name
	}
	return t.Name
}

// Field represents a field in a struct.
type Field struct {
	// Name is the field name.
	Name string
	// Type is the field type.
	Type *Type
	// Tag is the struct tag.
	Tag string
	// IsEmbedded indicates if it's an embedded field.
	IsEmbedded bool
}

// TypeKind represents the kind of a type.
type TypeKind int

const (
	TypeKindUnknown TypeKind = iota
	TypeKindPrimitive
	TypeKindStruct
	TypeKindInterface
	TypeKindMap
	TypeKindChan
	TypeKindFunc
	TypeKindSlice
	TypeKindArray
	TypeKindPointer
)

// ConversionTask represents a single conversion task between two types.
type ConversionTask struct {
	// SourceType is the source type to convert from.
	SourceType *Type
	// TargetType is the target type to convert to.
	TargetType *Type
	// Direction is the conversion direction.
	Direction ConversionDirection
	// RuleSet contains the rules for this conversion.
	RuleSet interface{} // Will be *config.RuleSet after proper integration

	// IsAlias indicates if this task is to generate a type alias.
	IsAlias bool
	// AliasName is the name of the alias to be generated (e.g., "DepartmentPB").
	AliasName string
}

// ConversionDirection represents the direction of conversion.
type ConversionDirection string

const (
	DirectionTo   ConversionDirection = "to"
	DirectionFrom ConversionDirection = "from"
	DirectionBoth ConversionDirection = "both"
)
