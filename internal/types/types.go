// Package types implements the functions, types, and interfaces for the module.
package types

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings" // Added this line
)

const (
	Application = "abgen"
	Description = "Alias Binding Generator is a tool for generating code for conversion between two types"
	WebSite     = "https://github.com/origadmin/abgen"
	UI          = `
   _____ ___. 
  /  _  \_ |__    ____   ____   ____
 /  /_\  \| __ \  / ___\_/ __ \ /    \
/    |    \ \_\ \/ /_/  >  ___/|   |  \
\____|__  /___  /\___  / \___  >___|  /
        \/    \//_____/      \/     \/
`
)

// TypeKind represents the kind of type for conversion helpers.
type TypeKind int

const (
	TypeKindUnknown TypeKind = iota
	TypeKindBasic
	TypeKindSlice
	TypeKindMap
	TypeKindStruct
	// Add more as needed
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
	IsPointer bool // Added for remap and nil checks
}

// Import represents a single Go import statement.
type Import struct {
	Alias string
	Path  string
}

type FieldConversion struct {
	Name         string
	Ignore       bool
	IgnoreReason string
	Conversion   string
}

// TypeInfo 类型信息
type TypeInfo struct {
	Name        string                 // 类型名称 (e.g., "User", "string", "Role") - base name only
	PkgName     string                 // 包名称 (例如 "po", "system")
	ImportPath  string                 // 导入路径
	ImportAlias string                 // 在当前代码中的导入别名 (例如 "typespb", "ent")
	LocalAlias  string                 // The local type alias name if this TypeInfo represents a local alias
	Fields      []StructField          // 结构体字段 (for struct types only)
	FieldsMap   map[string]StructField // 字段映射，方便通过字段名查找 (for struct types only)
	AliasFor    string                 // 类型别名目标
	IsAlias     bool                   // 是否是类型别名

	// New fields to represent complex types
	Kind        TypeKind   // The fundamental kind of type (e.g., Struct, Slice, Map, Basic)
	IsPointer   bool       // True if this type is a pointer (e.g., *User)
	IsSlice     bool       // True if this type is a slice (e.g., []User)
	IsMap       bool       // True if this type is a map (e.g., map[string]User)
	ElemType    *TypeInfo  // For pointers and slices, the element type
	KeyType     *TypeInfo  // For maps, the key type
	ValueType   *TypeInfo  // For maps, the value type
}

// BuildFieldsMap populates the FieldsMap for quick lookup.
func (ti *TypeInfo) BuildFieldsMap() {
	if ti.FieldsMap == nil {
		ti.FieldsMap = make(map[string]StructField)
	}
	for _, field := range ti.Fields {
		ti.FieldsMap[strings.ToLower(field.Name)] = field
	}
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

// ImportManager defines the interface for managing imports and aliases during code generation.
type ImportManager interface {
	GetType(pkgPath, typeName string) string
	GetImports() []Import // Change to types.Import
	RegisterAlias(alias string)
	IsAliasRegistered(alias string) bool
}

// Concrete implementation of ImportManager
type importManager struct {
	imports           map[string]string
	aliases           map[string]string
	localPackage      string
	registeredAliases map[string]bool
}

// NewImportManager creates and returns a new instance of importManager.
func NewImportManager(localPackage string) *importManager {
	return &importManager{
		imports:           make(map[string]string),
		aliases:           make(map[string]string),
		localPackage:      localPackage,
		registeredAliases: make(map[string]bool),
	}
}

// GetType returns the aliased type name for a given package path and type name.
func (im *importManager) GetType(pkgPath, typeName string) string {
	if pkgPath == "" || pkgPath == im.localPackage {
		return typeName
	}
	alias, exists := im.imports[pkgPath]
	if !exists {
		base := filepath.Base(pkgPath)
		alias = base
		for i := 1; im.aliases[alias] != "" && im.aliases[alias] != pkgPath; i++ {
			alias = fmt.Sprintf("%s%d", base, i)
		}
		im.imports[pkgPath] = alias
		im.aliases[alias] = pkgPath
	}
	return fmt.Sprintf("%s.%s", alias, typeName)
}

// GetImports returns a slice of types.Import representing the collected imports.
func (im *importManager) GetImports() []Import { // Return []Import (the struct in types.go)
	var imports []Import // Use Import (the struct in types.go)
	for path, alias := range im.imports {
		imports = append(imports, Import{Alias: alias, Path: path}) // Use Import (the struct in types.go)
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i].Path < imports[j].Path })
	return imports
}

// RegisterAlias registers an alias to prevent conflicts.
func (im *importManager) RegisterAlias(alias string) { im.registeredAliases[alias] = true }

// IsAliasRegistered checks if an alias is already registered.
func (im *importManager) IsAliasRegistered(alias string) bool { return im.registeredAliases[alias] }