package model

import (
	"bytes"

	"github.com/origadmin/abgen/internal/config"
)

// GenerationContext 包含代码生成所需的所有上下文信息
type GenerationContext struct {
	Config           *config.Config
	TypeInfos        map[string]*TypeInfo
	InvolvedPackages map[string]struct{}
}

// GenerationRequest 表示代码生成请求
type GenerationRequest struct {
	Context *GenerationContext
}

// GenerationResponse 表示代码生成响应
type GenerationResponse struct {
	GeneratedCode    []byte
	CustomStubs      []byte
	RequiredPackages []string
}

// TypeResolver 定义类型解析接口
type TypeResolver interface {
	ResolveType(fqn string) (*TypeInfo, error)
	ResolveAll(packagePaths []string, fqns []string) (map[string]*TypeInfo, error)
}

// CodeGenerator 定义代码生成接口
type CodeGenerator interface {
	Generate(request *GenerationRequest) (*GenerationResponse, error)
}

// ImportManager 定义导入管理接口
type ImportManager interface {
	Add(importPath string) string
	GetAlias(importPath string) string
	GetAllImports() []string
}

// TypeConverter 定义类型转换接口
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

// NameGenerator 定义命名生成接口
type NameGenerator interface {
	GetFunctionName(source, target *TypeInfo) string
	GetPrimitiveConversionStubName(parentSource *TypeInfo, sourceField *FieldInfo,
		parentTarget *TypeInfo, targetField *FieldInfo) string
	GetAlias(info *TypeInfo, isSource bool) string
	GetTypeString(info *TypeInfo) string
	GetTypeAliasString(info *TypeInfo) string
	PopulateSourcePkgs(config *config.Config)
}

// AliasManager 定义别名管理接口
type AliasManager interface {
	EnsureTypeAlias(typeInfo *TypeInfo, isSource bool)
	GetAliasMap() map[string]string
	GetRequiredAliases() map[string]struct{}
}

// ConversionEngine 定义转换引擎接口
type ConversionEngine interface {
	GenerateConversionFunction(sourceInfo, targetInfo *TypeInfo, rule *config.ConversionRule) error
	GenerateSliceConversion(sourceInfo, targetInfo *TypeInfo) error
	GetConversionExpression(parentSource *TypeInfo, sourceField *FieldInfo,
		parentTarget *TypeInfo, targetField *FieldInfo, fromVar string) (string, bool, bool)
}

// CodeEmitter 定义代码输出接口
type CodeEmitter interface {
	EmitHeader(buf *bytes.Buffer) error
	EmitImports(buf *bytes.Buffer, imports []string) error
	EmitAliases(buf *bytes.Buffer) error
	EmitConversions(buf *bytes.Buffer) error
	EmitHelpers(buf *bytes.Buffer) error
}