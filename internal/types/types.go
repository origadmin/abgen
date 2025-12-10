// Package types implements the functions, types, and interfaces for the module.
package types

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

// Import represents a single Go import statement.
type Import struct {
	Alias string
	Path  string
}

// ImportManager defines the interface for managing imports and aliases during code generation.
type ImportManager interface {
	GetType(pkgPath, typeName string) string
	GetImports() []Import
	RegisterAlias(alias string)
	IsAliasRegistered(alias string) bool
}

// IsPrimitiveType checks if a type name is a Go primitive type.
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

// IsNumberType checks if a type name is a numeric type.
func IsNumberType(t string) bool {
	numberTypes := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
	}
	return numberTypes[t]
}
