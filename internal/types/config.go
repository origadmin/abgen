// Package types implements the functions, types, and interfaces for the module.
package types

// TypeConversionRule defines a rule for converting between two specific types.
type TypeConversionRule struct {
	SourceTypeName string // The name of the source type (e.g., "Gender", "time.Time").
	TargetTypeName string // The name of the target type (e.g., "string", "timestamp.Timestamp").
	ConvertFunc    string // The name of the custom conversion function to use.
	// Direction   string // Optional: "to", "from", or "both" if different functions are used for each direction.
	// For now, we assume a single rule implies a specific direction or is part of a pair.
}

// ConversionConfig 转换配置
type ConversionConfig struct {
	SourceType       string
	TargetType       string
	Direction        string // both/to/from
	IgnoreFields     map[string]bool
	// FieldFuncs       map[string]string // Old: Replaced by FieldConversionRules
	// FieldConversionRules []FieldConversionRule // Old: Replaced by TypeConversionRules
	TypeConversionRules []TypeConversionRule // New: Stores detailed type conversion rules.
	SrcPackage       string
	DstPackage       string
	SourcePrefix     string
	SourceSuffix     string
	TargetPrefix     string
	TargetSuffix     string
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
