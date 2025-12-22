package model

import (
	"bytes"
	"go/types"

	"github.com/origadmin/abgen/internal/config"
)

// --- Generation Flow Structs ---

// GenerationResponse represents the final output of a code generation task.
type GenerationResponse struct {
	GeneratedCode    []byte
	CustomStubs      []byte
	RequiredPackages []string
}

// GeneratedCode holds information about a generated code snippet.
type GeneratedCode struct {
	FunctionBody    string
	RequiredHelpers []string
}

// AliasRenderInfo holds information for rendering a type alias.
type AliasRenderInfo struct {
	AliasName        string
	OriginalTypeName string
}

// --- Core Component Interfaces ---

// CodeGenerator defines the top-level interface for the code generation orchestrator.
type CodeGenerator interface {
	Generate(cfg *config.Config, typeInfos map[string]*TypeInfo) (*GenerationResponse, error)
}

// ImportManager defines the interface for managing and generating import statements.
type ImportManager interface {
	Add(pkgPath string) string
	AddAs(pkgPath, alias string) string
	GetAlias(pkgPath string) (string, bool)
	GetAllImports() map[string]string
	PackageName(pkg *types.Package) string
}

// AliasLookup defines the interface for looking up type aliases.
type AliasLookup interface {
	LookupAlias(uniqueKey string) (string, bool)
}

// NameGenerator defines the interface for generating correct Go syntax for names.
type NameGenerator interface {
	// ConversionFunctionName returns a standardized name for a function that converts between two types.
	ConversionFunctionName(source, target *TypeInfo) string
	// FieldConversionFunctionName returns a standardized name for a function that converts a specific field
	// between two parent structs.
	FieldConversionFunctionName(sourceParent, targetParent *TypeInfo, sourceField, targetField *FieldInfo) string
}

// AliasManager defines the interface for creating and managing local type aliases.
type AliasManager interface {
	AliasLookup // Embed AliasLookup interface
	PopulateAliases()
	GetAllAliases() map[string]string
	GetAliasedTypes() map[string]*TypeInfo
	GetAlias(info *TypeInfo) (string, bool)
	IsUserDefined(uniqueKey string) bool
	GetSourcePath() string
	GetTargetPath() string
}

// ConversionEngine defines the interface for the component that generates the body
// of conversion functions.
type ConversionEngine interface {
	GenerateConversionFunction(source, target *TypeInfo, rule *config.ConversionRule) (*GeneratedCode, []*ConversionTask, error)
	GenerateSliceConversion(source, target *TypeInfo) (*GeneratedCode, error)
	GetStubsToGenerate() map[string]*ConversionTask
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

// TypeConverter defines the interface for utility functions that inspect TypeInfo objects.
// Most type checking methods are removed in favor of direct access to info.Kind.
type TypeConverter interface {
	GetElementType(info *TypeInfo) *TypeInfo
	GetSliceElementType(info *TypeInfo) *TypeInfo
	GetKeyType(info *TypeInfo) *TypeInfo
	IsUltimatelyPrimitive(info *TypeInfo) bool
	IsPurelyPrimitiveOrCompositeOfPrimitives(info *TypeInfo) bool
}
