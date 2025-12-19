package model

import (
	"bytes"

	"github.com/origadmin/abgen/internal/config"
)

// --- Generation Flow Structs ---

// GenerationResponse represents the final output of a code generation task.
type GenerationResponse struct {
	GeneratedCode    []byte
	CustomStubs      []byte
	RequiredPackages []string
}

// GeneratedCode holds a piece of generated code, like a function body,
// and metadata about its dependencies.
type GeneratedCode struct {
	FunctionBody    string
	RequiredHelpers []string
}

// AliasRenderInfo holds the necessary information for rendering a type alias
// in the generated file.
type AliasRenderInfo struct {
	AliasName        string
	OriginalTypeName string
}

// --- Core Component Interfaces ---

// CodeGenerator defines the top-level interface for the code generation orchestrator.
type CodeGenerator interface {
	// Generate executes a complete code generation task based on the given configuration
	// and resolved type information.
	Generate(cfg *config.Config, typeInfos map[string]*TypeInfo) (*GenerationResponse, error)
}

// ImportManager defines the interface for managing and generating import statements.
type ImportManager interface {
	// Add ensures a package is imported and returns the alias that should be used for it.
	Add(pkgPath string) string
	// AddAs ensures a package is imported with a specific alias.
	AddAs(pkgPath, alias string) string
	// GetAlias retrieves the alias for a given package path, if it has been imported.
	GetAlias(pkgPath string) (string, bool)
	// GetAllImports returns a map of all imported package paths to their aliases.
	GetAllImports() map[string]string
}

// NameGenerator defines the interface for generating correct Go syntax for type and function names.
type NameGenerator interface {
	// TypeName returns the string representation of a type, including its package qualifier,
	// suitable for use in generated Go code.
	TypeName(info *TypeInfo) string
	// ConversionFunctionName returns a standardized name for a function that converts
	// from a source type to a target type.
	ConversionFunctionName(source, target *TypeInfo) string
}

// AliasManager defines the interface for creating and managing local type aliases.
type AliasManager interface {
	// PopulateAliases scans the configuration and type information to determine
	// which aliases need to be created.
	PopulateAliases()
	// GetSourceAlias retrieves the generated alias for a source type.
	GetSourceAlias(info *TypeInfo) string
	// GetTargetAlias retrieves the generated alias for a target type.
	GetTargetAlias(info *TypeInfo) string
	// GetAllAliases returns a map of all managed aliases, mapping a type's unique key to its alias.
	GetAllAliases() map[string]string
	// GetAliasesToRender returns a list of aliases that need to be written to the generated file.
	GetAliasesToRender() []*AliasRenderInfo
}

// ConversionEngine defines the interface for the component that generates the body
// of conversion functions.
type ConversionEngine interface {
	GenerateConversionFunction(source, target *TypeInfo, rule *config.ConversionRule) (*GeneratedCode, error)
	GenerateSliceConversion(source, target *TypeInfo) (*GeneratedCode, error)
}

// CodeEmitter defines the interface for writing the various sections of the final
// generated Go file.
type CodeEmitter interface {
	EmitHeader(buf *bytes.Buffer) error
	EmitImports(buf *bytes.Buffer, imports map[string]string) error
	EmitAliases(buf *bytes.Buffer, aliases []*AliasRenderInfo) error
	EmitConversions(buf *bytes.Buffer, funcs []string) error
	EmitHelpers(buf *bytes.Buffer, helpers map[string]struct{}) error
}

// TypeConverter defines the interface for utility functions that inspect and
// convert TypeInfo objects.
type TypeConverter interface {
	Convert(typeInfo *TypeInfo, pkgQualifier func(string) string) string
	IsSlice(info *TypeInfo) bool
	IsPointer(info *TypeInfo) bool
	IsStruct(info *TypeInfo) bool
	GetElementType(info *TypeInfo) *TypeInfo
}
