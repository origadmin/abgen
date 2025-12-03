// Package types implements the functions, types, and interfaces for the module.
package types

const (
	Application = "abgen"
	Description = "Alias Binding Generator is a tool for generating code for conversion between two types"
	WebSite     = "https://github.com/origadmin/abgen"
	UI          = `
   _____ ___.
  /  _  \\_ |__    ____   ____   ____
 /  /_\  \| __ \  / ___\_/ __ \ /    \
/    |    \ \_\ \/ /_/  >  ___/|   |  \
\____|__  /___  /\___  / \___  >___|  /
        \/    \//_____/      \/     \/
`
)

// FieldMapping Structs are used to store field mapping information
type FieldMapping struct {
	GoName     string // Go 字段名
	PbName     string // PB 字段名
	CustomFunc string // 自定义转换函数
}

// TypeBinding Structs are used to store type binding information
type TypeBinding struct {
	GoType string // Go 类型
	PbType string // PB 类型
	Fields []FieldMapping
}
type StructField struct {
	Name     string
	Type     string
	Exported bool
}

type FieldConversion struct {
	Name         string
	Ignore       bool
	IgnoreReason string
	Conversion   string
}

// TypeInfo 类型信息
type TypeInfo struct {
	Name       string        // 类型名称
	PkgName    string        // 包名称 (例如 "po", "system")
	ImportPath string        // 导入路径
	Fields     []StructField // 结构体字段
	AliasFor   string        // 类型别名目标
	IsAlias    bool          // 是否是类型别名
}

// IsPrimitiveType 判断是否为原始类型
func IsPrimitiveType(t string) bool {
	primitiveTypes := map[string]bool{
		"bool":   true,
		"string": true,
		"int":    true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
		"byte": true, "rune": true,
	}
	return primitiveTypes[t]
}

// IsNumberType 判断是否为数字类型
func IsNumberType(t string) bool {
	numberTypes := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
	}
	return numberTypes[t]
}
