package model

import (
	"bytes"

	"github.com/origadmin/abgen/internal/config"
)

// GenerationContext contains all the context information needed for code generation.
type GenerationContext struct {
	Config           *config.Config
	TypeInfos        map[string]*TypeInfo
	InvolvedPackages map[string]struct{}
}

// GenerationRequest represents a code generation request.
type GenerationRequest struct {
	Context *GenerationContext
}

// GenerationResponse represents a code generation response.
type GenerationResponse struct {
	GeneratedCode    []byte
	CustomStubs      []byte
	RequiredPackages []string
}

// CodeGenerator defines the interface for the main code generator.
type CodeGenerator interface {
	Generate(request *GenerationRequest) (*GenerationResponse, error)
}

// TypeResolver defines the interface for type resolution.
type TypeResolver interface {
	ResolveType(fqn string) (*TypeInfo, error)
	ResolveAll(packagePaths []string, fqns []string) (map[string]*TypeInfo, error)
}

// ImportManager defines the interface for import management.
type ImportManager interface {
	Add(importPath string) string
	GetAlias(importPath string) string
	GetAllImports() []string
}

// TypeConverter defines the interface for type conversion.
type TypeConverter interface {
	IsPointer(info *TypeInfo) bool
	GetElementType(info *TypeInfo) *TypeInfo
	GetKeyType(info *TypeInfo) *TypeInfo
	IsStruct(info *TypeInfo) bool
	IsSlice(info *TypeInfo) bool
	IsArray(info *TypeInfo) bool
	IsMap(info *TypeInfo) bool
	IsPrimitive(info *TypeInfo) bool
	IsUltimatelyPrimitive(info *TypeInfo) bool
}

// NameGenerator defines the interface for name generation.
type NameGenerator interface {
	GetFunctionName(source, target *TypeInfo) string
	GetPrimitiveConversionStubName(parentSource *TypeInfo, sourceField *FieldInfo,
		parentTarget *TypeInfo, targetField *FieldInfo) string
	GetAlias(info *TypeInfo, isSource bool) string
	GetTypeString(info *TypeInfo) string
	GetTypeAliasString(info *TypeInfo) string
	PopulateSourcePkgs(config *config.Config)
}

// AliasRenderInfo holds the necessary information for rendering a type alias.
type AliasRenderInfo struct {
	AliasName        string
	OriginalTypeName string
}

// AliasManager defines the interface for alias management.
type AliasManager interface {
	EnsureTypeAlias(typeInfo *TypeInfo, isSource bool)
	GetAliasMap() map[string]string
	SetFieldTypesToAlias(fieldTypes map[string]*TypeInfo)
	GetRequiredAliases() map[string]struct{}
	PopulateAliases()
	GetAliasesToRender() []*AliasRenderInfo
}

// GeneratedCode holds the generated code from the conversion engine.
type GeneratedCode struct {
	FunctionBody    string
	RequiredHelpers []string
}

// ConversionEngine defines the interface for the conversion engine.
type ConversionEngine interface {
	GenerateConversionFunction(sourceInfo, targetInfo *TypeInfo, rule *config.ConversionRule) (*GeneratedCode, error)
	GenerateSliceConversion(sourceInfo, targetInfo *TypeInfo) (*GeneratedCode, error)
	GetConversionExpression(parentSource *TypeInfo, sourceField *FieldInfo,
		parentTarget *TypeInfo, targetField *FieldInfo, fromVar string) (string, bool, bool, []string)
}

// CodeEmitter defines the interface for the code emitter.
type CodeEmitter interface {
	EmitHeader(buf *bytes.Buffer) error
	EmitImports(buf *bytes.Buffer, imports []string) error
	EmitAliases(buf *bytes.Buffer) error
	EmitConversions(buf *bytes.Buffer, conversions []string) error
	EmitHelpers(buf *bytes.Buffer, requiredHelpers map[string]struct{}) error
}
