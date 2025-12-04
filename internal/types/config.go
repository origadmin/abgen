// Package types implements the functions, types, and interfaces for the module.
package types

// ConversionConfig 转换配置
type ConversionConfig struct {
	SourceType   string
	TargetType   string
	Direction    string // both/to/from
	IgnoreFields map[string]bool
	SrcPackage   string
	DstPackage   string
	SourcePrefix string
	SourceSuffix string
	TargetPrefix string
	TargetSuffix string
	GeneratorPkgPath string // The package path where the generation is happening
}

// ConversionNode 类型转换节点
type ConversionNode struct {
	FromConversions []string // 该类型可作为源类型的转换目标
	ToConversions   []string // 该类型可作为目标类型的来源
	Configs         map[string]*ConversionConfig
}

// ConversionGraph 类型转换关系图
type ConversionGraph map[string]*ConversionNode

// PackageConversionConfig holds the configuration for converting all matching types between two packages.
type PackageConversionConfig struct {
	SourcePackage string
	TargetPackage string
	Direction     string
	IgnoreTypes   map[string]bool
	FieldMap      map[string]string
	SourcePrefix  string
	SourceSuffix  string
	TargetPrefix  string
	TargetSuffix  string
}
